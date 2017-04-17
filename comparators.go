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

	"k8s.io/client-go/pkg/api/meta"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/runtime"
)

type EqualFunc func(a, b runtime.Object) bool

func EqualUIDs(a, b runtime.Object) bool {
	metaA, _ := meta.Accessor(a)
	metaB, _ := meta.Accessor(b)

	return metaA.GetUID() == metaB.GetUID()
}

func EqualResourceVersion(a, b runtime.Object) bool {
	metaA, _ := meta.Accessor(a)
	metaB, _ := meta.Accessor(b)

	return metaA.GetResourceVersion() == metaB.GetResourceVersion()
}

func getEndpointsUIDs(e *v1.Endpoints) map[string]bool {
	uids := make(map[string]bool)
	for _, subset := range e.Subsets {
		for _, port := range subset.Ports {
			for _, address := range subset.Addresses {
				uids[fmt.Sprintf("%s:%d", address.IP, port.Port)] = true
			}
		}
	}
	return uids
}

func EqualEndpoints(a, b runtime.Object) bool {
	endpointsA, ok := a.(*v1.Endpoints)
	if !ok {
		return false
	}

	endpointsB, ok := b.(*v1.Endpoints)
	if !ok {
		return false
	}

	if endpointsA.UID == endpointsB.UID && endpointsA.ResourceVersion == endpointsB.ResourceVersion {
		return true
	}

	uidsA := getEndpointsUIDs(endpointsA)
	uidsB := getEndpointsUIDs(endpointsB)

	if len(uidsA) != len(uidsB) {
		return false
	}

	for uid := range uidsA {
		if _, found := uidsB[uid]; !found {
			return false
		}
	}

	return true
}
