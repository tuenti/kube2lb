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

	domain string
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

func NewKubernetesClient(kubecfg, apiserver, domain string) (*KubernetesClient, error) {
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
			domain:    domain,
		}, nil
	}
}

func (c *KubernetesClient) AddNotifier(n Notifier) {
	c.notifiers = append(c.notifiers, n)
}

func (c *KubernetesClient) Notify() {
	for _, n := range c.notifiers {
		if err := n.Notify(); err != nil {
			log.Printf("Couldn't notify: %s\n", err)
		}
	}
}

func (c *KubernetesClient) AddTemplate(t *Template) {
	c.templates = append(c.templates, t)
}

func (c *KubernetesClient) ExecuteTemplates(info *ClusterInformation) {
	for _, t := range c.templates {
		if err := t.Execute(info); err != nil {
			log.Printf("Couldn't write template: %s\n", err)
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
		return nil, fmt.Errorf("Couldn't get services: ", err)
	}
	servicesInformation := make([]ServiceInformation, 0, len(services.Items))
	for _, s := range services.Items {
		switch s.Spec.Type {
		case api.ServiceTypeNodePort, api.ServiceTypeLoadBalancer:
			for _, port := range s.Spec.Ports {
				servicesInformation = append(servicesInformation,
					ServiceInformation{
						Name:      s.Name,
						Namespace: s.Namespace,
						Port:      port.Port,
						NodePort:  port.NodePort,
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
		return fmt.Errorf("Couldn't get nodes: ", err)
	}

	services, err := c.getServices(api.NamespaceAll)
	if err != nil {
		return fmt.Errorf("Couldn't get services: ", err)
	}

	info := &ClusterInformation{
		Nodes:    nodeNames,
		Services: services,
		Domain:   c.domain,
	}
	c.ExecuteTemplates(info)
	c.Notify()

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

	isFirstUpdate := true
	updater := NewUpdater(func() {
		var err error
		if err = c.Update(); err != nil {
			log.Printf("Couldn't update state: ", err)
		}
		if isFirstUpdate {
			if err != nil {
				log.Fatalf("Failing on first update, check configuration: ", err)
			}
			isFirstUpdate = false
		}
	})
	go updater.Run()

	for {
		select {
		case e := <-nodeWatcher.ResultChan():
			if e.Type == watch.Added || e.Type == watch.Deleted {
				updater.Signal()
			}
		case <-serviceWatcher.ResultChan():
			updater.Signal()
		}
	}
}
