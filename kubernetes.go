/*
Copyright 2017 Tuenti Technologies S.L. All rights reserved.

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
	"net"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var defaultLBIP = net.IPv4zero.String()
var defaultPortMode = "http"
var reconnectTimeoutSeconds = 300

func init() {
	flag.StringVar(&defaultLBIP, "default-lb-ip", defaultLBIP, "Default IP for services in load balancer, can be overriden by loadBalancerIP service field")
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

	lastResourceVersion string

	updaterBuilder UpdaterBuilder
	eventForwarder func(watch.Event)

	notifiers []Notifier
	templates []Template

	domain string
}

const (
	ExternalDomainsAnnotation = "kube2lb/external-domains"
	PortModeAnnotation        = "kube2lb/port-mode"
	BackendTimeoutAnnotation  = "kube2lb/backend-timeout"
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
		config:         config,
		clientset:      clientset,
		notifiers:      make([]Notifier, 0, 10),
		templates:      make([]Template, 0, 10),
		domain:         domain,
		updaterBuilder: NewUpdater,
	}

	if err := kc.connect(); err != nil {
		return nil, err
	}
	return kc, nil
}

func (c *KubernetesClient) connect() (err error) {
	log.Printf("Using %s for kubernetes master", c.config.Host)

	options := meta_v1.ListOptions{
		ResourceVersion: c.lastResourceVersion,
	}

	defer func() {
		if err != nil {
			c.stopWatchers()
		}
	}()

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

func (c *KubernetesClient) stopWatchers() {
	if c.nodeWatcher != nil {
		c.nodeWatcher.Stop()
	}
	if c.serviceWatcher != nil {
		c.serviceWatcher.Stop()
	}
	if c.endpointsWatcher != nil {
		c.endpointsWatcher.Stop()
	}
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

func (c *KubernetesClient) AddTemplate(t Template) {
	c.templates = append(c.templates, t)
}

func (c *KubernetesClient) ExecuteTemplates(info *ClusterInformation) {
	for _, t := range c.templates {
		if err := t.Execute(info); err != nil {
			log.Printf("Couldn't write template: %s", err)
		}
	}
}

func (c *KubernetesClient) readAnnotation(meta meta_v1.ObjectMeta, annotation string, value interface{}) {
	data, ok := meta.Annotations[annotation]
	if ok && len(data) > 0 {
		err := json.Unmarshal([]byte(data), value)
		if err != nil {
			log.Printf("Couldn't parse %s annotation for %s service: %s", annotation, meta.Name, err)
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
		c.readAnnotation(s.ObjectMeta, PortModeAnnotation, &portModes)

		var backendTimeouts map[string]int
		c.readAnnotation(s.ObjectMeta, BackendTimeoutAnnotation, &backendTimeouts)

		switch s.Spec.Type {
		case v1.ServiceTypeNodePort, v1.ServiceTypeLoadBalancer:
			endpointsPortsMap := endpointsHelper.ServicePortsMap(s)
			if len(endpointsPortsMap) == 0 {
				log.Printf("Couldn't find endpoints for %s in %s?", s.Name, s.Namespace)
				continue
			}

			err := ValidateService(s)
			if err != nil {
				log.Printf("Service validation failed: %s", err)
				break
			}

			parsedLBIP := net.ParseIP(defaultLBIP)
			if s.Spec.Type == v1.ServiceTypeLoadBalancer && s.Spec.LoadBalancerIP != "" {
				parsedLBIP = net.ParseIP(s.Spec.LoadBalancerIP)
			}

			for _, port := range s.Spec.Ports {
				mode, ok := portModes[port.Name]
				if !ok {
					mode = defaultPortMode
				}
				timeout, ok := backendTimeouts[port.Name]
				if !ok {
					timeout = 0
				}
				servicesInformation = append(servicesInformation,
					ServiceInformation{
						Name:      s.Name,
						Namespace: s.Namespace,
						Port: PortSpec{
							IP:       parsedLBIP,
							Port:     port.Port,
							Mode:     strings.ToLower(mode),
							Protocol: strings.ToLower(string(port.Protocol)),
						},
						Endpoints: endpointsPortsMap[port.TargetPort.IntVal],
						NodePort:  port.NodePort,
						External:  external,
						Timeout:   timeout,
					},
				)
			}
		}
	}
	return servicesInformation, nil
}

func (c *KubernetesClient) Update() error {
	nodeNames := c.nodeStore.GetNames()

	if net.ParseIP(defaultLBIP) == nil {
		return fmt.Errorf("invalid default lb IP %s", defaultLBIP)
	}

	services, err := c.getServices()
	if err != nil {
		return fmt.Errorf("couldn't get services: %s", err)
	}

	portsMap := make(map[string]PortSpec)
	for _, service := range services {
		portsMap[service.Port.String()] = service.Port
	}
	ports := make([]PortSpec, 0, len(portsMap))
	for _, port := range portsMap {
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
	updater := c.updaterBuilder(func() {
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

	resetStores := func() {
		isFirstUpdate = true
		c.nodeStore = NodeStore{NewLocalStore()}
		c.serviceStore = ServiceStore{NewLocalStore()}
		c.endpointsStore = EndpointsStore{NewLocalStore()}
		c.lastResourceVersion = ""
	}
	resetStores()

	updateStore := func(s Store, e watch.Event) {
		switch e.Type {
		case watch.Added:
			s.Update(e.Object)
		case watch.Modified:
			old := s.Update(e.Object)
			if old == nil {
				log.Println("Modified unknown object, this shouldn't happen")
				break
			}
			eq, err := s.Equal(old, e.Object)
			if err != nil {
				log.Println(err)
				return
			}
			if eq {
				return
			}
		case watch.Deleted:
			s.Delete(e.Object)
		case watch.Error:
			status, ok := e.Object.(*meta_v1.Status)
			if ok {
				log.Printf("Error received while watching: %s", status.Message)
			}
			log.Println("Local caches will be rebuilt")
			resetStores()
			return
		}
		accessor, _ := meta.Accessor(e.Object)
		if accessor != nil {
			c.lastResourceVersion = accessor.GetResourceVersion()
		}
		updater.Signal()
	}

	var more bool
	var e watch.Event
	for {
		select {
		case e, more = <-c.nodeWatcher.ResultChan():
			updateStore(c.nodeStore, e)
		case e, more = <-c.serviceWatcher.ResultChan():
			updateStore(c.serviceStore, e)
		case e, more = <-c.endpointsWatcher.ResultChan():
			updateStore(c.endpointsStore, e)
		}

		// Used in tests to know when events have been processed
		if c.eventForwarder != nil {
			c.eventForwarder(e)
		}

		if !more || e.Type == watch.Error {
			log.Printf("Connection closed, trying to reconnect...")
			timeout := time.Duration(reconnectTimeoutSeconds) * time.Second
			err := wait.Poll(5*time.Second, timeout, func() (bool, error) {
				err := c.connect()
				if err != nil {
					log.Println("Couldn't reconnect: ", err)
				}
				return err == nil, nil
			})
			if err != nil {
				return err
			}
		}
	}
}
