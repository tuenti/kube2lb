package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"

	"github.com/jsoriano/getsignal"
)

func main() {
	var (
		signalName string
		pid        int
	)

	flag.StringVar(&signalName, "signal", "", "Signal name")
	flag.IntVar(&pid, "pid", 0, "PID to signal")
	flag.Parse()

	signal, err := getsignal.FromName(signalName)
	if err != nil {
		fmt.Printf("Unknown signal\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Printf("Signaling %d with signal number %d\n", pid, signal)
	if err := syscall.Kill(pid, signal); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
