package main

import (
	"fmt"
	"log"
	"time"

	api "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

type KubernetesClient struct {
	client *client.Client

	notifiers []Notifier
	templates []*Template
}

func NewKubernetesClient(apiserver string) (*KubernetesClient, error) {
	var err error
	c := &KubernetesClient{
		notifiers: make([]Notifier, 0, 10),
		templates: make([]*Template, 0, 10),
	}

	c.client, err = client.New(&client.Config{
		Host: apiserver,
	})
	if err != nil {
		return nil, err
	}
	return c, nil
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
	nodes, err := ni.List(options)
	if err != nil {
		return fmt.Errorf("Couldn't get nodes: ", err)
	}

	si := c.client.Services(api.NamespaceAll)
	services, err := si.List(options)
	if err != nil {
		return fmt.Errorf("Couldn't get services: ", err)
	}

	nodeNames := make([]string, len(nodes.Items))
	for i, n := range nodes.Items {
		nodeNames[i] = n.Name
	}
	servicePorts := make([]ServicePorts, 0, len(services.Items))
	for _, s := range services.Items {
		if s.Spec.Type == api.ServiceTypeNodePort || s.Spec.Type == api.ServiceTypeLoadBalancer {
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

func (c *KubernetesClient) Watch() {
	for {
		if err := c.Update(); err != nil {
			log.Printf("Couldn't update state: ", err)
		}
		time.Sleep(5 * time.Second)
	}
}
