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

	api "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/watch"
)

var defaultPortMode = "http"

func init() {
	flag.StringVar(&defaultPortMode, "default-port-mode", defaultPortMode, "Default mode for service ports")
}

type KubernetesClient struct {
	client       *client.Client
	clientConfig *client.Config

	nodeWatcher    watch.Interface
	serviceWatcher watch.Interface

	notifiers []Notifier
	templates []*Template

	domain string
}

const (
	ExternalDomainsAnnotation = "kube2lb/external-domains"
	PortModeAnnotation        = "kube2lb/port-mode"
)

func getClientConfig(kubecfg, apiserver string) (*client.Config, error) {
	if apiserver == "" && kubecfg == "" {
		if config, err := client.InClusterConfig(); err == nil {
			return config, nil
		} else {
			return nil, err
		}
	}
	if apiserver != "" && kubecfg == "" {
		return &client.Config{Host: apiserver}, nil
	}
	overrides := &clientcmd.ConfigOverrides{}
	overrides.ClusterInfo.Server = apiserver
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = kubecfg
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
}

func NewKubernetesClient(kubecfg, apiserver, domain string) (*KubernetesClient, error) {
	var (
		config *client.Config
		err    error
	)

	if config, err = getClientConfig(kubecfg, apiserver); err != nil {
		return nil, err
	}

	kc := &KubernetesClient{
		clientConfig: config,
		notifiers:    make([]Notifier, 0, 10),
		templates:    make([]*Template, 0, 10),
		domain:       domain,
	}

	if err := kc.connect(); err != nil {
		return nil, err
	}
	return kc, nil
}

func (c *KubernetesClient) connect() (err error) {
	log.Printf("Using %s for kubernetes master", c.clientConfig.Host)

	if c.client, err = client.New(c.clientConfig); err != nil {
		return
	}

	options := api.ListOptions{}

	ni := c.client.Nodes()
	c.nodeWatcher, err = ni.Watch(options.LabelSelector, options.FieldSelector, "")
	if err != nil {
		return fmt.Errorf("Couldn't watch events on nodes: %v", err)
	}

	si := c.client.Services(api.NamespaceAll)
	c.serviceWatcher, err = si.Watch(options.LabelSelector, options.FieldSelector, "")
	if err != nil {
		return fmt.Errorf("Couldn't watch events on services: %v", err)
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
	ni := c.client.Nodes()
	nodes, err := ni.List(options.LabelSelector, options.FieldSelector)
	if err != nil {
		return nil, err
	}
	nodeNames := make([]string, len(nodes.Items))
	for i, n := range nodes.Items {
		nodeNames[i] = n.Name
	}
	return nodeNames, nil
}

func (c *KubernetesClient) getServices(namespace string) ([]ServiceInformation, error) {
	options := api.ListOptions{}
	si := c.client.Services(api.NamespaceAll)
	services, err := si.List(options.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get services: %s", err)
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
		switch s.Spec.Type {
		case api.ServiceTypeNodePort, api.ServiceTypeLoadBalancer:
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
						NodePort: port.NodePort,
						External: external,
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
