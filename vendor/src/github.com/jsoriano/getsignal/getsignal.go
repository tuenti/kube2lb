package getsignal

import (
	"fmt"
	"syscall"
)

func FromName(name string) (syscall.Signal, error) {
	signal, ok := sigMapping[name]
	if !ok {
		return -1, fmt.Errorf("Unknown signal name: %s", name)
	}
	return signal, nil
}
