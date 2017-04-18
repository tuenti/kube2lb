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
	"fmt"
	"testing"
	"time"

	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
	"k8s.io/client-go/pkg/watch"
)

type testWatcher struct {
	resultChan chan watch.Event
}

func newTestWatcher() *testWatcher {
	return &testWatcher{make(chan watch.Event)}
}

func (*testWatcher) Stop() {}

func (w *testWatcher) ResultChan() <-chan watch.Event {
	return w.resultChan
}

type testNotifier struct {
	waitChan chan struct{}
}

func newTestNotifier() *testNotifier {
	return &testNotifier{make(chan struct{})}
}

func (n *testNotifier) Notify() error {
	n.waitChan <- struct{}{}
	return nil
}

func (n *testNotifier) Wait() error {
	select {
	case <-n.waitChan:
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("notify timeout")
	}
}

type dummyTemplate struct {
	executionCount   int
	lastExecutedWith *ClusterInformation
}

func (t *dummyTemplate) Execute(info *ClusterInformation) error {
	t.executionCount++
	t.lastExecutedWith = info
	return nil
}

func TestKubernetesWatch(t *testing.T) {
	nodeWatcher := newTestWatcher()
	serviceWatcher := newTestWatcher()
	endpointsWatcher := newTestWatcher()

	client := &KubernetesClient{
		nodeWatcher:      nodeWatcher,
		serviceWatcher:   serviceWatcher,
		endpointsWatcher: endpointsWatcher,
		domain:           "kube2lb.test",
	}
	notifier := newTestNotifier()
	client.AddNotifier(notifier)

	template := &dummyTemplate{}
	client.AddTemplate(template)

	go client.Watch()

	nodeEvents := []watch.Event{
		watch.Event{
			Type: watch.Added,
			Object: &v1.Node{
				ObjectMeta: v1.ObjectMeta{SelfLink: "/node/1", Name: "node1", UID: "1"},
			},
		},
		watch.Event{
			Type: watch.Added,
			Object: &v1.Node{
				ObjectMeta: v1.ObjectMeta{SelfLink: "/node/2", Name: "node2", UID: "2"},
			},
		},
		watch.Event{
			Type: watch.Deleted,
			Object: &v1.Node{
				ObjectMeta: v1.ObjectMeta{SelfLink: "/node/1", Name: "node1", UID: "1"},
			},
		},
	}

	serviceEvents := []watch.Event{
		watch.Event{
			Type: watch.Added,
			Object: &v1.Service{
				ObjectMeta: v1.ObjectMeta{SelfLink: "/service/1", Name: "service1", Namespace: "test", ResourceVersion: "1"},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeNodePort,
					Ports: []v1.ServicePort{
						{
							Name: "http", Port: 80, TargetPort: intstr.FromInt(80),
						},
					},
				},
			},
		},
		watch.Event{
			Type: watch.Added,
			Object: &v1.Service{
				ObjectMeta: v1.ObjectMeta{SelfLink: "/service/2", Name: "service2", Namespace: "test", ResourceVersion: "1"},
				Spec:       v1.ServiceSpec{Type: v1.ServiceTypeClusterIP},
			},
		},
	}

	endpointsEvents := []watch.Event{
		watch.Event{
			Type: watch.Added,
			Object: &v1.Endpoints{
				ObjectMeta: v1.ObjectMeta{SelfLink: "/endpoints/1", Name: "service1", Namespace: "test", ResourceVersion: "1"},
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}},
						Ports:     []v1.EndpointPort{{Name: "http", Port: 80}},
					},
				},
			},
		},
		watch.Event{
			Type: watch.Modified,
			Object: &v1.Endpoints{
				ObjectMeta: v1.ObjectMeta{SelfLink: "/endpoints/1", Name: "service1", Namespace: "test", ResourceVersion: "2"},
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{{IP: "10.0.0.1"}},
						Ports:     []v1.EndpointPort{{Name: "http", Port: 80}},
					},
				},
			},
		},
	}

	for _, event := range nodeEvents {
		nodeWatcher.resultChan <- event
	}

	for _, event := range serviceEvents {
		serviceWatcher.resultChan <- event
	}

	for _, event := range endpointsEvents {
		endpointsWatcher.resultChan <- event
	}

	err := notifier.Wait()
	if err != nil {
		t.Fatal(err)
	}

	if template.executionCount == 0 {
		t.Fatal("template not executed")
	}

	info := template.lastExecutedWith
	if info == nil {
		t.Fatal("template executed without cluster information?")
	}
	if len(info.Nodes) != 1 {
		t.Fatalf("%d nodes in cluster information, %d expected", len(info.Nodes), 1)
	}
	if info.Nodes[0] != "node2" {
		t.Fatalf("%s node found, %s expected", info.Nodes[0], "node2")
	}
	if len(info.Services) != 1 {
		t.Fatalf("%d services in cluster information, %d expected", len(info.Services), 1)
	}
	if len(info.Services[0].Endpoints) != 1 {
		t.Fatalf("%d endpoints for service, %d expected", len(info.Services[0].Endpoints), 1)
	}
	if info.Services[0].Endpoints[0].IP != "10.0.0.1" {
		t.Fatalf("%s endpoint for service, %s expected", info.Services[0].Endpoints[0].IP, "10.0.0.1")
	}
}
