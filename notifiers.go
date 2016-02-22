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
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/jsoriano/getsignal"
)

type Notifier interface {
	Notify() error
}

func NewNotifier(definition string) (Notifier, error) {
	ds := strings.SplitN(definition, ":", 2)
	if len(ds) < 2 {
		return nil, fmt.Errorf("Notifier definition expected")
	}
	t := ds[0]
	d := ds[1]
	switch t {
	case "command":
		return NewCommandNotifier(d)
	case "pid":
		return NewPidNotifier(d)
	case "pidfile":
		return NewPidfileNotifier(d)
	case "debug":
		return &DebugNotifier{}, nil
	default:
		return nil, fmt.Errorf("Don't know how to notify to '%s'", definition)
	}
}

type CommandNotifier struct {
	command string
}

func NewCommandNotifier(definition string) (*CommandNotifier, error) {
	// -notify command:COMMAND
	return &CommandNotifier{definition}, nil
}

func (n *CommandNotifier) Notify() error {
	cmd := exec.Command("/bin/sh", "-c", n.command)
	output, err := cmd.CombinedOutput()
	log.Printf("%s", output)
	return err
}

type PidNotifier struct {
	pid    int
	signal syscall.Signal
}

func NewPidNotifier(definition string) (*PidNotifier, error) {
	// -notify pid:SIGNAL:PID
	ds := strings.SplitN(definition, ":", 2)
	if len(ds) < 2 {
		return nil, fmt.Errorf("Missing arguments for PID notifier, expected: pid:SIGNAL:PID")
	}
	signal, err := getsignal.FromName(ds[0])
	if err != nil {
		return nil, err
	}
	pid, err := strconv.Atoi(ds[1])
	if err != nil {
		return nil, err
	}
	return &PidNotifier{pid: pid, signal: signal}, nil
}

func (n *PidNotifier) Notify() error {
	return syscall.Kill(n.pid, n.signal)
}

type PidfileNotifier struct {
	pidfile string
	signal  syscall.Signal
}

func NewPidfileNotifier(definition string) (*PidfileNotifier, error) {
	// -notify pidfile:SIGNAL:PIDFILE
	ds := strings.SplitN(definition, ":", 2)
	if len(ds) < 2 {
		return nil, fmt.Errorf("Missing arguments for PID notifier, expected: pidfile:SIGNAL:PIDFILE")
	}
	signal, err := getsignal.FromName(ds[0])
	if err != nil {
		return nil, err
	}
	pidfile := ds[1]
	return &PidfileNotifier{pidfile: pidfile, signal: signal}, nil
}

func (n *PidfileNotifier) Notify() error {
	c, err := ioutil.ReadFile(n.pidfile)
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.Trim(string(c), "\n\t "))
	if err != nil {
		return err
	}
	return syscall.Kill(pid, n.signal)
}

type DebugNotifier struct{}

func (n *DebugNotifier) Notify() error {
	log.Printf("Notify")
	return nil
}
