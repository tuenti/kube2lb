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
	"fmt"

	"k8s.io/client-go/pkg/api/v1"
)

type ServiceEndpoint struct {
	Name string
	IP   string
	Port int32
}

func (e *ServiceEndpoint) String() string {
	return fmt.Sprintf("%s:%d", e.IP, e.Port)
}

type EndpointsHelper struct {
	endpointsMap map[string]*v1.Endpoints
}

func metaKey(meta v1.ObjectMeta) string {
	return fmt.Sprintf("%s %s", meta.Name, meta.Namespace)
}

func NewEndpointsHelper(endpoints *v1.EndpointsList) *EndpointsHelper {
	endpointsMap := make(map[string]*v1.Endpoints)
	for i, endpoint := range endpoints.Items {
		endpointsMap[metaKey(endpoint.ObjectMeta)] = &endpoints.Items[i]
	}
	return &EndpointsHelper{endpointsMap}
}

func (h *EndpointsHelper) ServicePortsMap(s *v1.Service) map[int32][]ServiceEndpoint {
	endpoints, found := h.endpointsMap[metaKey(s.ObjectMeta)]
	if !found {
		return nil
	}
	m := make(map[int32][]ServiceEndpoint)
	for _, subset := range endpoints.Subsets {
		for _, port := range subset.Ports {
			var addresses []ServiceEndpoint
			for _, address := range subset.Addresses {
				if address.IP == "" {
					continue
				}
				name := address.IP
				if address.TargetRef != nil {
					name = address.TargetRef.Name
				}
				addresses = append(addresses, ServiceEndpoint{
					Name: name,
					IP:   address.IP,
					Port: port.Port,
				})
			}
			m[port.Port] = addresses
		}
	}
	return m
}
