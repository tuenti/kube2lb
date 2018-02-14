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
	"flag"
	"log"
	"sync/atomic"
	"time"
)

var updateTimeout float64

func init() {
	flag.Float64Var(&updateTimeout, "update-timeout", 10, "Update timeout in seconds")
}

type Updater interface {
	Run()
	Signal()
}

type UpdaterFunc func()

type UpdaterBuilder func(f UpdaterFunc) Updater

type antiBurstUpdater struct {
	updateNeeded  atomic.Value
	signal, burst chan struct{}
	f             UpdaterFunc
}

func NewUpdater(f UpdaterFunc) Updater {
	return &antiBurstUpdater{
		signal: make(chan struct{}),
		burst:  make(chan struct{}),
		f:      f,
	}
}

func (u *antiBurstUpdater) antiBurst() {
	for {
		select {
		case <-u.burst:
		case <-time.After(time.Second):
			if u.updateNeeded.Load().(int) == 1 {
				u.signal <- struct{}{}
			}
		}
	}
}

func (u *antiBurstUpdater) Run() {
	go u.antiBurst()
	for _ = range u.signal {
		u.updateNeeded.Store(0)

		c := make(chan struct{}, 1)
		go func() {
			u.f()
			c <- struct{}{}
		}()

		select {
		case <-c:
		case <-time.After(time.Duration(updateTimeout) * time.Second):
			log.Println("Update timed out")
		}
	}
}

func (u *antiBurstUpdater) Signal() {
	u.updateNeeded.Store(1)
	u.burst <- struct{}{}
}
