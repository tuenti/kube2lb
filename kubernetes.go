package main

import (
	"fmt"
	"log"

	api "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/watch"
)

type KubernetesClient struct {
	client *client.Client

	notifiers []Notifier
	templates []*Template
}

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

func NewKubernetesClient(kubecfg, apiserver string) (*KubernetesClient, error) {
	var (
		config *client.Config
		err    error
	)

	if config, err = getClientConfig(kubecfg, apiserver); err != nil {
		return nil, err
	}

	log.Printf("Using %s for kubernetes master", config.Host)
	if c, err := client.New(config); err != nil {
		return nil, err
	} else {
		return &KubernetesClient{
			client:    c,
			notifiers: make([]Notifier, 0, 10),
			templates: make([]*Template, 0, 10),
		}, nil
	}
}

func (c *KubernetesClient) AddNotifier(n Notifier) {
	c.notifiers = append(c.notifiers, n)
}

func (c *KubernetesClient) AddTemplate(t *Template) {
	c.templates = append(c.templates, t)
}

func (c *KubernetesClient) Update() error {
	options := api.ListOptions{}

	ni := c.client.Nodes()
	nodes, err := ni.List(options.LabelSelector, options.FieldSelector)
	if err != nil {
		return fmt.Errorf("Couldn't get nodes: ", err)
	}

	si := c.client.Services(api.NamespaceAll)
	services, err := si.List(options.LabelSelector)
	if err != nil {
		return fmt.Errorf("Couldn't get services: ", err)
	}

	nodeNames := make([]string, len(nodes.Items))
	for i, n := range nodes.Items {
		nodeNames[i] = n.Name
	}
	servicePorts := make([]ServicePorts, 0, len(services.Items))
	for _, s := range services.Items {
		switch s.Spec.Type {
		case api.ServiceTypeNodePort, api.ServiceTypeLoadBalancer:
			for _, port := range s.Spec.Ports {
				servicePorts = append(servicePorts, ServicePorts{s.Name, port.Port, port.NodePort})
			}
		}
	}

	info := &ClusterInformation{
		Nodes:        nodeNames,
		ServicePorts: servicePorts,
	}
	for _, t := range c.templates {
		if err := t.Execute(info); err != nil {
			log.Printf("Couldn't write template: %s\n", err)
		}
	}
	for _, n := range c.notifiers {
		if err := n.Notify(); err != nil {
			log.Printf("Couldn't notify: %s\n", err)
		}
	}

	return nil
}

func (c *KubernetesClient) Watch() error {
	options := api.ListOptions{}
	ni := c.client.Nodes()
	si := c.client.Services(api.NamespaceAll)

	nodeWatcher, err := ni.Watch(options.LabelSelector, options.FieldSelector, "")
	if err != nil {
		return fmt.Errorf("Couldn't watch events on nodes: %v", err)
	}

	serviceWatcher, err := si.Watch(options.LabelSelector, options.FieldSelector, "")
	if err != nil {
		return fmt.Errorf("Couldn't watch events on nodes: %v", err)
	}

	for {
		select {
		case e := <-nodeWatcher.ResultChan():
			if e.Type == watch.Added || e.Type == watch.Deleted {
				if err := c.Update(); err != nil {
					log.Printf("Couldn't update state: ", err)
				}
			}
		case <-serviceWatcher.ResultChan():
			if err := c.Update(); err != nil {
				log.Printf("Couldn't update state: ", err)
			}
		}
	}
}
