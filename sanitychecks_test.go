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

	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
)

func TestOnePortInRange(t *testing.T) {
	r := EphemeralPortsRange{check: true, LowPort: 20000, HighPort: 40000}
	s := &v1.Service{
		ObjectMeta: v1.ObjectMeta{SelfLink: "/service/1", Name: "service1", Namespace: "test", ResourceVersion: "3"},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{
					Name: "foo", Port: 20001, TargetPort: intstr.FromInt(20001),
				},
			},
		},
	}

	err := r.ValidateService(s)
	if err == nil {
		t.Fatalf("Port is in range and check enabled, should return err")
	}
}

func TestOnePortNotInRange(t *testing.T) {
	r := EphemeralPortsRange{check: true, LowPort: 20000, HighPort: 40000}
	s := &v1.Service{
		ObjectMeta: v1.ObjectMeta{SelfLink: "/service/1", Name: "service1", Namespace: "test", ResourceVersion: "3"},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{
					Name: "foo", Port: 19999, TargetPort: intstr.FromInt(19999),
				},
			},
		},
	}

	err := r.ValidateService(s)
	if err != nil {
		t.Fatalf("Port is not range and check enabled, should return nil")
	}
}

func TestMultiPortInRange(t *testing.T) {
	r := EphemeralPortsRange{check: true, LowPort: 20000, HighPort: 40000}
	s := &v1.Service{
		ObjectMeta: v1.ObjectMeta{SelfLink: "/service/1", Name: "service1", Namespace: "test", ResourceVersion: "3"},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{
					Name: "foo1", Port: 20001, TargetPort: intstr.FromInt(20001),
				},
				{
					Name: "foo2", Port: 20002, TargetPort: intstr.FromInt(20002),
				},
			},
		},
	}

	err := r.ValidateService(s)
	if err == nil {
		t.Fatalf("Ports are in range and check enabled, should return err")
	}
}

func TestMultiPortAllNotInRange(t *testing.T) {
	r := EphemeralPortsRange{check: true, LowPort: 20000, HighPort: 40000}
	s := &v1.Service{
		ObjectMeta: v1.ObjectMeta{SelfLink: "/service/1", Name: "service1", Namespace: "test", ResourceVersion: "3"},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{
					Name: "foo1", Port: 19998, TargetPort: intstr.FromInt(19998),
				},
				{
					Name: "foo2", Port: 19999, TargetPort: intstr.FromInt(19999),
				},
			},
		},
	}

	err := r.ValidateService(s)
	if err != nil {
		t.Fatalf("Ports are not in range and check enabled, should return nil")
	}
}

func TestMultiPortOneInRange(t *testing.T) {
	r := EphemeralPortsRange{check: true, LowPort: 20000, HighPort: 40000}
	s := &v1.Service{
		ObjectMeta: v1.ObjectMeta{SelfLink: "/service/1", Name: "service1", Namespace: "test", ResourceVersion: "3"},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{
					Name: "foo1", Port: 20001, TargetPort: intstr.FromInt(20001),
				},
				{
					Name: "foo2", Port: 19999, TargetPort: intstr.FromInt(19999),
				},
			},
		},
	}

	err := r.ValidateService(s)
	if err == nil {
		t.Fatalf("One port is in range and check enabled, should return err")
	}
}

func TestSanityCheckDisabled(t *testing.T) {
	r := EphemeralPortsRange{check: false, LowPort: 0, HighPort: 0}
	s := &v1.Service{
		ObjectMeta: v1.ObjectMeta{SelfLink: "/service/1", Name: "service1", Namespace: "test", ResourceVersion: "3"},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{
					Name: "foo1", Port: 20001, TargetPort: intstr.FromInt(20001),
				},
				{
					Name: "foo2", Port: 19999, TargetPort: intstr.FromInt(19999),
				},
			},
		},
	}

	err := r.ValidateService(s)
	if err != nil {
		t.Fatalf("Checks are disabled, should return nil")
	}
}
