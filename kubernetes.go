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

	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/api"
	"k8s.io/client-go/1.4/pkg/api/v1"
	"k8s.io/client-go/1.4/pkg/watch"
	"k8s.io/client-go/1.4/rest"
	"k8s.io/client-go/1.4/tools/clientcmd"
)

var defaultPortMode = "http"

func init() {
	flag.StringVar(&defaultPortMode, "default-port-mode", defaultPortMode, "Default mode for service ports")
}

type KubernetesClient struct {
	config    *rest.Config
	clientset *kubernetes.Clientset

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

	options := api.ListOptions{}

	ni := c.clientset.Core().Nodes()
	c.nodeWatcher, err = ni.Watch(options)
	if err != nil {
		return fmt.Errorf("Couldn't watch events on nodes: %v", err)
	}

	si := c.clientset.Core().Services(api.NamespaceAll)
	c.serviceWatcher, err = si.Watch(options)
	if err != nil {
		return fmt.Errorf("Couldn't watch events on services: %v", err)
	}

	ei := c.clientset.Core().Endpoints(api.NamespaceAll)
	c.endpointsWatcher, err = ei.Watch(options)
	if err != nil {
		return fmt.Errorf("Couldn't watch events on endpoints: %v", err)
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

func (c *KubernetesClient) getNodeNames() ([]string, error) {
	options := api.ListOptions{}
	ni := c.clientset.Core().Nodes()
	nodes, err := ni.List(options)
	if err != nil {
		return nil, err
	}
	nodeNames := make([]string, len(nodes.Items))
	for i, n := range nodes.Items {
		nodeNames[i] = n.Name
	}
	return nodeNames, nil
}

func (c *KubernetesClient) getEndpointsMap(endpoints *v1.Endpoints) map[int32][]string {
	m := make(map[int32][]string)
	for _, subset := range endpoints.Subsets {
		for _, port := range subset.Ports {
			var addresses []string
			for _, address := range subset.Addresses {
				if address.IP == "" {
					continue
				}
				addresses = append(addresses, fmt.Sprintf("%s:%d", address.IP, port.Port))
			}
			m[port.Port] = addresses
		}
	}
	return m
}

func (c *KubernetesClient) getServices(namespace string) ([]ServiceInformation, error) {
	options := api.ListOptions{}

	si := c.clientset.Core().Services(api.NamespaceAll)
	services, err := si.List(options)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get services: %s", err)
	}

	ei := c.clientset.Core().Endpoints(api.NamespaceAll)
	endpoints, err := ei.List(options)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get endpoints: %s", err)
	}

	endpointsMap := make(map[string]*v1.Endpoints)
	for i, endpoint := range endpoints.Items {
		k := fmt.Sprintf("%s_%s", endpoint.Namespace, endpoint.Name)
		endpointsMap[k] = &endpoints.Items[i]
	}

	servicesInformation := make([]ServiceInformation, 0, len(services.Items))
	for _, s := range services.Items {
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
		endpointsKey := fmt.Sprintf("%s_%s", s.Namespace, s.Name)
		endpoints, found := endpointsMap[endpointsKey]
		if !found {
			log.Printf("Couldn't find endpoints for %s in %s?", s.Name, s.Namespace)
			continue
		}

		endpointsPortsMap := c.getEndpointsMap(endpoints)

		switch s.Spec.Type {
		case v1.ServiceTypeNodePort, v1.ServiceTypeLoadBalancer:
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
	nodeNames, err := c.getNodeNames()
	if err != nil {
		return fmt.Errorf("Couldn't get nodes: %s", err)
	}

	services, err := c.getServices(api.NamespaceAll)
	if err != nil {
		return fmt.Errorf("Couldn't get services: %s", err)
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

	var more bool
	var e watch.Event
	for {
		select {
		case e, more = <-c.nodeWatcher.ResultChan():
			if e.Type == watch.Added || e.Type == watch.Deleted {
				updater.Signal()
			}
		case _, more = <-c.serviceWatcher.ResultChan():
			updater.Signal()
		case e, more = <-c.endpointsWatcher.ResultChan():
			updater.Signal()
		}
		if !more {
			log.Printf("Connection closed, trying to reconnect")
			for {
				if err := c.connect(); err != nil {
					log.Println(err)
					time.Sleep(5 * time.Second)
				} else {
					break
				}
			}
		}
	}
}
