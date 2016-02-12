package main

import (
	"sync/atomic"
	"time"
)

type Updater struct {
	updateNeeded  atomic.Value
	signal, burst chan struct{}
	f             func()
}

func NewUpdater(f func()) *Updater {
	return &Updater{
		signal: make(chan struct{}),
		burst:  make(chan struct{}),
		f:      f,
	}
}

func (u *Updater) antiBurst() {
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

func (u *Updater) Run() {
	go u.antiBurst()
	for _ = range u.signal {
		u.updateNeeded.Store(0)
		u.f()
	}
}

func (u *Updater) Signal() {
	u.updateNeeded.Store(1)
	u.burst <- struct{}{}
}
