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
	"log"
	"net"
	"time"

	"github.com/achanda/go-sysctl"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	addressesExpiration = 5 * time.Second

	ephemeralPortsRangeSysKey = "net.ipv4.ip_local_port_range"
	nonLocalBindSysKey        = "net.ipv4.ip_nonlocal_bind"
)

type ServiceValidator interface {
	ValidateService(*v1.Service) error
}

var (
	sanityChecks []ServiceValidator
)

func ValidateService(s *v1.Service) error {
	for _, check := range sanityChecks {
		if err := check.ValidateService(s); err != nil {
			return err
		}
	}
	return nil
}

type EphemeralPortsRange struct {
	check    bool
	LowPort  int32
	HighPort int32
}

// Provide the range as a String
func (r EphemeralPortsRange) String() string {
	return fmt.Sprintf("%d->%d", r.LowPort, r.HighPort)
}

// Return error if any of the ports is in the ephemeral ports range, otherwise return nil (also if the check is disabled)
func (r EphemeralPortsRange) ValidateService(s *v1.Service) error {
	// Check is disabled
	if !r.check {
		return nil
	}

	for _, port := range s.Spec.Ports {
		// Port found in range
		if (port.Port >= r.LowPort) && (port.Port <= r.HighPort) {
			return fmt.Errorf("service %s in %s Service Port %d is in the ephemeral ports range (%s), skipping it. Please check your configuration!", s.Name, s.Namespace, port.Port, r)
		}
	}
	// None of the ports in range
	return nil
}

// Retrieve the data from sysctl and return the values, disable the check if unsuccessful and log
func initEphemeralPortsRangeCheck() *EphemeralPortsRange {
	var l, h int32
	r, err := sysctl.Get(ephemeralPortsRangeSysKey)
	if err != nil {
		log.Printf("Error reading %s from sysctl: %s, skipping ephemeral ports range checks", ephemeralPortsRangeSysKey, err)
		return &EphemeralPortsRange{check: false, LowPort: 0, HighPort: 0}
	}

	fmt.Sscanf(r, "%d %d", &l, &h)
	return &EphemeralPortsRange{check: true, LowPort: l, HighPort: h}
}

func init() {
	sanityChecks = append(sanityChecks, initEphemeralPortsRangeCheck())
}

type AddressForLoadBalancerIP struct {
	checkLocalBind     bool
	interfaceAddresses []net.Addr
	addressesTime      time.Time
}

func (a *AddressForLoadBalancerIP) addresses() ([]net.Addr, error) {
	if a.addressesTime.IsZero() || time.Since(a.addressesTime) > addressesExpiration {
		addresses, err := net.InterfaceAddrs()
		if err != nil {
			return nil, err
		}
		a.addressesTime = time.Now()
		a.interfaceAddresses = addresses
	}
	return a.interfaceAddresses, nil
}

func (a AddressForLoadBalancerIP) ValidateService(s *v1.Service) error {
	if s.Spec.Type != v1.ServiceTypeLoadBalancer || s.Spec.LoadBalancerIP == "" {
		return nil
	}

	ip := net.ParseIP(s.Spec.LoadBalancerIP)
	if ip == nil {
		return fmt.Errorf("couldn't parse IP '%s' for service %s in %s",
			s.Spec.LoadBalancerIP, s.Name, s.Namespace)
	}

	if a.checkLocalBind {
		addrs, err := a.addresses()
		if err != nil {
			log.Printf("Error obtaining local interface addresses: %s", err)
			return nil
		}
		for _, addr := range addrs {
			switch addr := addr.(type) {
			case *net.IPNet:
				if addr.IP.Equal(ip) {
					return nil
				}
			}
		}
		return fmt.Errorf("service %s in %s cannot be bound to address %s defined in load balancer IP, skipping it. Please check your configuration!", s.Name, s.Namespace, s.Spec.LoadBalancerIP)
	}

	return nil
}

func initAddressForLoadBalancerIPCheck() *AddressForLoadBalancerIP {
	nonLocalBind, err := sysctl.Get(nonLocalBindSysKey)
	if err != nil {
		log.Printf("Error reading %s from sysctl: %s, skipping load balancer IP checks", nonLocalBindSysKey, err)
		return &AddressForLoadBalancerIP{checkLocalBind: false}
	}

	return &AddressForLoadBalancerIP{checkLocalBind: nonLocalBind == "0"}
}

func init() {
	sanityChecks = append(sanityChecks, initAddressForLoadBalancerIPCheck())
}
