/*
Copyright 2016 Tuenti Technologies S.L. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var defaultPortMode = "http"
var reconnectTimeoutSeconds = 300

func init() {
	flag.StringVar(&defaultPortMode, "default-port-mode", defaultPortMode, "Default mode for service ports")
	flag.IntVar(&reconnectTimeoutSeconds, "reconnect-timeout", reconnectTimeoutSeconds, "Reconnect timeout in seconds")
}

type KubernetesClient struct {
	config    *rest.Config
	clientset *kubernetes.Clientset

	nodeStore      NodeStore
	serviceStore   ServiceStore
	endpointsStore EndpointsStore

	nodeWatcher      watch.Interface
	serviceWatcher   watch.Interface
	endpointsWatcher watch.Interface

	notifiers []Notifier
	templates []*Template

	domain string
}

const (
	ExternalDomainsAnnotation = "kube2lb/external-domains"
	PortModeAnnotation        = "kube2lb/port-mode"
)

func NewKubernetesClient(kubecfg, apiserver, domain string) (*KubernetesClient, error) {
	config, err := clientcmd.BuildConfigFromFlags(apiserver, kubecfg)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	kc := &KubernetesClient{
		config:    config,
		clientset: clientset,
		notifiers: make([]Notifier, 0, 10),
		templates: make([]*Template, 0, 10),
		domain:    domain,
	}

	if err := kc.connect(); err != nil {
		return nil, err
	}
	return kc, nil
}

func (c *KubernetesClient) connect() (err error) {
	log.Printf("Using %s for kubernetes master", c.config.Host)

	options := v1.ListOptions{}

	ni := c.clientset.Core().Nodes()
	c.nodeWatcher, err = ni.Watch(options)
	if err != nil {
		return fmt.Errorf("couldn't watch events on nodes: %v", err)
	}

	si := c.clientset.Core().Services(api.NamespaceAll)
	c.serviceWatcher, err = si.Watch(options)
	if err != nil {
		return fmt.Errorf("couldn't watch events on services: %v", err)
	}

	ei := c.clientset.Core().Endpoints(api.NamespaceAll)
	c.endpointsWatcher, err = ei.Watch(options)
	if err != nil {
		return fmt.Errorf("couldn't watch events on endpoints: %v", err)
	}
	return
}

func (c *KubernetesClient) AddNotifier(n Notifier) {
	c.notifiers = append(c.notifiers, n)
}

func (c *KubernetesClient) Notify() {
	for _, n := range c.notifiers {
		if err := n.Notify(); err != nil {
			log.Printf("Couldn't notify: %s", err)
		}
	}
}

func (c *KubernetesClient) AddTemplate(t *Template) {
	c.templates = append(c.templates, t)
}

func (c *KubernetesClient) ExecuteTemplates(info *ClusterInformation) {
	for _, t := range c.templates {
		if err := t.Execute(info); err != nil {
			log.Printf("Couldn't write template: %s", err)
		}
	}
}

func (c *KubernetesClient) getServices() ([]ServiceInformation, error) {
	services, err := c.serviceStore.List()
	if err != nil {
		return nil, fmt.Errorf("couldn't get services: %s", err)
	}

	endpoints, err := c.endpointsStore.List()
	if err != nil {
		return nil, fmt.Errorf("couldn't get endpoints: %s", err)
	}

	endpointsHelper := NewEndpointsHelper(endpoints)

	servicesInformation := make([]ServiceInformation, 0, len(services))
	for _, s := range services {
		var external []string
		if domains, ok := s.ObjectMeta.Annotations[ExternalDomainsAnnotation]; ok && len(domains) > 0 {
			external = strings.Split(domains, ",")
		}
		var portModes map[string]string
		if modes, ok := s.ObjectMeta.Annotations[PortModeAnnotation]; ok && len(modes) > 0 {
			err := json.Unmarshal([]byte(modes), &portModes)
			if err != nil {
				log.Printf("Couldn't parse %s annotation for %s service", PortModeAnnotation, s.Name)
			}
		}

		switch s.Spec.Type {
		case v1.ServiceTypeNodePort, v1.ServiceTypeLoadBalancer:
			endpointsPortsMap := endpointsHelper.ServicePortsMap(s)
			if len(endpointsPortsMap) == 0 {
				log.Printf("Couldn't find endpoints for %s in %s?", s.Name, s.Namespace)
				continue
			}

			for _, port := range s.Spec.Ports {
				mode, ok := portModes[port.Name]
				if !ok {
					mode = defaultPortMode
				}
				servicesInformation = append(servicesInformation,
					ServiceInformation{
						Name:      s.Name,
						Namespace: s.Namespace,
						Port: PortSpec{
							port.Port,
							strings.ToLower(mode),
							strings.ToLower(string(port.Protocol)),
						},
						Endpoints: endpointsPortsMap[port.TargetPort.IntVal],
						NodePort:  port.NodePort,
						External:  external,
					},
				)
			}
		}
	}
	return servicesInformation, nil
}

func (c *KubernetesClient) Update() error {
	nodeNames := c.nodeStore.GetNames()

	services, err := c.getServices()
	if err != nil {
		return fmt.Errorf("couldn't get services: %s", err)
	}

	portsMap := make(map[PortSpec]bool)
	for _, service := range services {
		portsMap[service.Port] = true
	}
	ports := make([]PortSpec, 0, len(portsMap))
	for port := range portsMap {
		ports = append(ports, port)
	}

	info := &ClusterInformation{
		Nodes:    nodeNames,
		Services: services,
		Ports:    ports,
		Domain:   c.domain,
	}
	c.ExecuteTemplates(info)
	c.Notify()

	return nil
}

func (c *KubernetesClient) Watch() error {
	isFirstUpdate := true
	updater := NewUpdater(func() {
		var err error
		if err = c.Update(); err != nil {
			log.Printf("Couldn't update state: %s", err)
		}
		if isFirstUpdate {
			if err != nil {
				log.Fatalf("Failing on first update, check configuration.")
			}
			isFirstUpdate = false
		}
	})
	go updater.Run()

	c.nodeStore = NodeStore{NewLocalStore()}
	c.serviceStore = ServiceStore{NewLocalStore()}
	c.endpointsStore = EndpointsStore{NewLocalStore()}

	updateStore := func(s Store, e watch.Event, equal EqualFunc) {
		if e.Object == nil {
			return
		}
		switch e.Type {
		case watch.Added, watch.Modified:
			// We handle Added the same as Modified because when reconnecting we
			// receive everything as Added
			old := s.Update(e.Object)
			if old == nil || !equal(old, e.Object) {
				updater.Signal()
			}
		case watch.Deleted:
			s.Delete(e.Object)
			updater.Signal()
		}
	}

	var more bool
	var e watch.Event
	for {
		select {
		case e, more = <-c.nodeWatcher.ResultChan():
			updateStore(c.nodeStore, e, EqualUIDs)
		case e, more = <-c.serviceWatcher.ResultChan():
			updateStore(c.serviceStore, e, func(a, b runtime.Object) bool {
				return EqualUIDs(a, b) && EqualResourceVersion(a, b)
			})
		case e, more = <-c.endpointsWatcher.ResultChan():
			updateStore(c.endpointsStore, e, EqualEndpoints)
		}

		if !more {
			log.Printf("Connection closed, trying to reconnect...")
			timeout := time.Duration(reconnectTimeoutSeconds) * time.Second
			err := wait.Poll(5*time.Second, timeout, func() (bool, error) {
				err := c.connect()
				if err != nil {
					log.Println(err)
				}
				return err == nil, nil
			})
			if err != nil {
				return err
			}
		}
	}
}
