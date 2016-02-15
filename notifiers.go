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
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"syscall"
)

type Notifier interface {
	Notify() error
}

func NewNotifier(definition string) (Notifier, error) {
	ds := strings.SplitN(definition, ":", 2)
	if len(ds) != 2 {
		return nil, fmt.Errorf("Unknown notifier definition")
	}

	t := ds[0]
	target := ds[1]
	switch t {
	case "pid":
		pid, err := strconv.Atoi(target)
		if err != nil {
			return nil, err
		}
		return &PidNotifier{Pid: pid, Signal: syscall.SIGUSR1}, nil
	case "pidfile":
		return &PidfileNotifier{Pidfile: target, Signal: syscall.SIGUSR1}, nil
	case "debug":
		return &DebugNotifier{}, nil
	default:
		return nil, fmt.Errorf("Don't know how to notify to '%s'", definition)
	}
}

type PidNotifier struct {
	Pid    int
	Signal syscall.Signal
}

func (n *PidNotifier) Notify() error {
	return syscall.Kill(n.Pid, n.Signal)
}

type PidfileNotifier struct {
	Pidfile string
	Signal  syscall.Signal
}

func (n *PidfileNotifier) Notify() error {
	c, err := ioutil.ReadFile(n.Pidfile)
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.Trim(string(c), "\n\t "))
	if err != nil {
		return err
	}
	return syscall.Kill(pid, n.Signal)
}

type DebugNotifier struct{}

func (n *DebugNotifier) Notify() error {
	log.Printf("Notify")
	return nil
}
