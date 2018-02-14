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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/pkg/api/v1"
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
	return &testNotifier{make(chan struct{}, 10)}
}

func (n *testNotifier) Notify(context.Context) error {
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

// An updater that doesn't call the updater function but register
// if it has been signaled
type dummyUpdater struct {
	Signaled bool
	F        UpdaterFunc
}

func (dummyUpdater) Run(ctx context.Context) {
	<-ctx.Done()
}

func (u *dummyUpdater) Signal() {
	u.Signaled = true
}

func (u *dummyUpdater) Build(f UpdaterFunc) Updater {
	u.F = f
	return u
}

func TestKubernetesWatch(t *testing.T) {
	nodeWatcher := newTestWatcher()
	serviceWatcher := newTestWatcher()
	endpointsWatcher := newTestWatcher()

	updater := dummyUpdater{}
	eventForwarderChan := make(chan struct{}, 100)
	consumeForwardedEvent := func() {
		select {
		case <-eventForwarderChan:
		case <-time.After(10 * time.Millisecond):
			t.Fatal("event consumption timeout")
		}
	}

	// Setup client
	client := &KubernetesClient{
		nodeWatcher:      nodeWatcher,
		serviceWatcher:   serviceWatcher,
		endpointsWatcher: endpointsWatcher,
		domain:           "kube2lb.test",
		updaterBuilder:   updater.Build,
		eventForwarder: func(watch.Event) {
			eventForwarderChan <- struct{}{}
		},
	}
	notifier := newTestNotifier()
	client.AddNotifier(notifier)

	template := &dummyTemplate{}
	client.AddTemplate(template)

	ctx := context.Background()
	go client.Watch(ctx)

	// Send events to watchers
	nodeEvents := []watch.Event{
		watch.Event{
			Type: watch.Added,
			Object: &v1.Node{
				ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/node/1", Name: "node1", UID: "1"},
			},
		},
		watch.Event{
			Type: watch.Added,
			Object: &v1.Node{
				ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/node/2", Name: "node2", UID: "2"},
			},
		},
		watch.Event{
			Type: watch.Deleted,
			Object: &v1.Node{
				ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/node/1", Name: "node1", UID: "1"},
			},
		},
	}

	serviceEvents := []watch.Event{
		watch.Event{
			Type: watch.Added,
			Object: &v1.Service{
				ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/service/1", Name: "service1", Namespace: "test", ResourceVersion: "3"},
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
				ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/service/2", Name: "service2", Namespace: "test", ResourceVersion: "4"},
				Spec:       v1.ServiceSpec{Type: v1.ServiceTypeClusterIP},
			},
		},
	}

	endpointsEvents := []watch.Event{
		watch.Event{
			Type: watch.Added,
			Object: &v1.Endpoints{
				ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/endpoints/1", Name: "service1", Namespace: "test", ResourceVersion: "5"},
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
				ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/endpoints/1", Name: "service1", Namespace: "test", ResourceVersion: "6"},
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{{IP: "10.0.0.1"}},
						Ports:     []v1.EndpointPort{{Name: "http", Port: 80}},
					},
				},
			},
		},
	}

	eventCount := 0
	for _, event := range nodeEvents {
		nodeWatcher.resultChan <- event
		eventCount++
	}

	for _, event := range serviceEvents {
		serviceWatcher.resultChan <- event
		eventCount++
	}

	for _, event := range endpointsEvents {
		endpointsWatcher.resultChan <- event
		eventCount++
	}

	for i := 0; i < eventCount; i++ {
		consumeForwardedEvent()
	}

	// Check effects
	assert.Equal(t, updater.Signaled, true, "Updater should have been signaled")
	updater.F(ctx)

	if template.executionCount == 0 {
		t.Fatal("template not executed")
	}

	info := template.lastExecutedWith
	if assert.NotNil(t, info, "template executed without cluster information?") {
		if assert.Equal(t, len(info.Nodes), 1, "expected number of nodes") {
			assert.Equal(t, info.Nodes[0], "node2", "expected name of first node")
		}
		if assert.Equal(t, len(info.Services), 1, "expected number of services") {
			if assert.Equal(t, len(info.Services[0].Endpoints), 1, "expected number of endpoints on first service") {
				assert.Equal(t, info.Services[0].Endpoints[0].IP, "10.0.0.1", "endpoint for first service")
			}
		}
	}

	// Send an event that wouldn't modify the state and check that updater is not signaled
	updater.Signaled = false
	nodeWatcher.resultChan <- watch.Event{
		Type: watch.Modified,
		Object: &v1.Node{
			ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/node/2", Name: "node2", UID: "2"},
		},
	}
	consumeForwardedEvent()
	assert.Equal(t, updater.Signaled, false, "Updater shouldn't have been signaled on repeated modification")

	// Adding an annotation to a service should signal updater
	updater.Signaled = false
	serviceWatcher.resultChan <- watch.Event{
		Type: watch.Modified,
		Object: &v1.Service{
			ObjectMeta: meta_v1.ObjectMeta{
				SelfLink:        "/service/1",
				Name:            "service1",
				Namespace:       "test",
				ResourceVersion: "7",
				Annotations:     map[string]string{ExternalDomainsAnnotation: "service1.example.com"},
			},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeNodePort,
				Ports: []v1.ServicePort{
					{
						Name: "http", Port: 80, TargetPort: intstr.FromInt(80),
					},
				},
			},
		},
	}
	consumeForwardedEvent()
	assert.Equal(t, updater.Signaled, true, "Updater should have been signaled when adding annotation to service")
	updater.F(ctx)
}
