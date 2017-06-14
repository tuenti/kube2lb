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
	"os"
	"os/signal"
	"path"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"
)

func init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR2)
	go func() {
		for range c {
			fileName, err := dumpMemProfile()
			if err != nil {
				log.Printf("Couldn't write memory profile: %s\n", err)
				continue
			}
			log.Printf("Memory profile dumped to %s", fileName)
		}
	}()
}

func dumpMemProfile() (string, error) {
	timestamp := time.Now().Format(time.RFC3339)
	profFileName := path.Join(os.TempDir(), fmt.Sprintf("kube2lb-memprof-%s", timestamp))
	f, err := os.Create(profFileName)
	if err != nil {
		return "", err
	}
	runtime.GC()
	err = pprof.WriteHeapProfile(f)
	if err != nil {
		return "", err
	}
	f.Close()
	return profFileName, nil
}
