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
	"testing"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
)

func TestUpdate(t *testing.T) {
	service1 := &v1.Service{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/foo/1", UID: "1"}}
	service2 := &v1.Service{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/foo/1", UID: "2"}}

	store := NewLocalStore()

	if store.Update(service1) != nil {
		t.Fatalf("Adding to empty storage shouldn't return anything")
	}

	old := store.Update(service2)
	if old != service1 {
		t.Fatalf("Updating with new object should return old object")
	}
	if old, ok := old.(*v1.Service); !ok || old.ObjectMeta.UID != service1.ObjectMeta.UID {
		t.Fatalf("Returned object is not original object")
	}

	if len(store.Objects) != 1 {
		t.Fatalf("Store should contain an only object at this point")
	}
}

func TestDelete(t *testing.T) {
	service1 := &v1.Service{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/foo/1", UID: "1"}}
	service2 := &v1.Service{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/foo/1", UID: "2"}}

	store := NewLocalStore()

	if store.Delete(service1) != nil {
		t.Fatalf("Deleting on empty store")
	}

	store.Update(service1)

	old := store.Delete(service2)
	if old != service1 {
		t.Fatalf("Deleting object with same SelfLink should return old object")
	}
	if old, ok := old.(*v1.Service); !ok || old.ObjectMeta.UID != service1.ObjectMeta.UID {
		t.Fatalf("Returned object is not original object")
	}

	if len(store.Objects) != 0 {
		t.Fatalf("Store shouldn't contain any object at this point")
	}
}

func TestGetNodeNames(t *testing.T) {
	nodes := []*v1.Node{
		&v1.Node{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/node/1", Name: "node1"}},
		&v1.Node{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/node/2", Name: "node2"}},
	}

	store := NodeStore{NewLocalStore()}

	for _, node := range nodes {
		store.Update(node)
	}

	names := store.GetNames()

	for _, node := range nodes {
		found := false
		for _, name := range names {
			if node.ObjectMeta.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Node name not found for %s", node.ObjectMeta.Name)
		}
	}
}

func TestListServices(t *testing.T) {
	services := []*v1.Service{
		&v1.Service{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/service/1", Name: "service1"}},
		&v1.Service{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/service/2", Name: "service2"}},
		&v1.Service{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/service/3", Name: "service3"}},
	}

	store := ServiceStore{NewLocalStore()}

	for _, service := range services {
		store.Update(service)
	}

	serviceList, err := store.List()
	if err != nil {
		t.Fatalf("Error when getting service list: %v", err)
	}

	for _, service := range services {
		found := false
		for _, serviceOnList := range serviceList {
			if service.ObjectMeta.Name == serviceOnList.ObjectMeta.Name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Service not found in list: %s", service.ObjectMeta.Name)
		}
	}
}

func TestListEndpoints(t *testing.T) {
	endpoints := []*v1.Endpoints{
		&v1.Endpoints{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/endpoints/1", Name: "endpoints1"}},
		&v1.Endpoints{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/endpoints/2", Name: "endpoints2"}},
		&v1.Endpoints{ObjectMeta: meta_v1.ObjectMeta{SelfLink: "/endpoints/3", Name: "endpoints3"}},
	}

	store := EndpointsStore{NewLocalStore()}

	for _, endpoint := range endpoints {
		store.Update(endpoint)
	}

	endpointList, err := store.List()
	if err != nil {
		t.Fatalf("Error when getting endpoint list: %v", err)
	}

	for _, endpoint := range endpoints {
		found := false
		for _, endpointOnList := range endpointList {
			if endpoint.ObjectMeta.Name == endpointOnList.ObjectMeta.Name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Endpoint not found in list: %s", endpoint.ObjectMeta.Name)
		}
	}
}
