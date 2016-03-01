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
	"fmt"
	"log"
	"os"
)

var version = "dev"

func main() {
	var apiserver, kubecfg, domain, configPath, templatePath, notify string
	var showVersion bool
	flag.StringVar(&apiserver, "apiserver", "", "Kubernetes API server URL")
	flag.StringVar(&kubecfg, "kubecfg", "", "Path to kubernetes client configuration (Optional)")
	flag.StringVar(&domain, "domain", "local", "DNS domain for the cluster")
	flag.StringVar(&configPath, "config", "", "Configuration path to generate")
	flag.StringVar(&templatePath, "template", "", "Configuration source template")
	flag.StringVar(&notify, "notify", "", "Kubernetes API server URL")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if _, err := os.Stat(templatePath); err != nil {
		log.Fatalf("Template not defined or doesn't exist")
	}

	if notify == "" {
		log.Fatalf("Notifier cannot be empty")
	}

	if f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE, 0644); err != nil {
		log.Fatalf("Cannot open configuration file to write: %v", err)
	} else {
		f.Close()
	}

	notifier, err := NewNotifier(notify)
	if err != nil {
		log.Fatalf("Couldn't initialize notifier: %s", err)
	}

	client, err := NewKubernetesClient(kubecfg, apiserver, domain)
	if err != nil {
		log.Fatalf("Couldn't connect with Kubernetes API server: %s", err)
	}

	if err := initServerNameTemplates(); err != nil {
		log.Fatalf("Couldn't initialize server name templates: %s", err)
	}

	client.AddTemplate(NewTemplate(templatePath, configPath))
	client.AddNotifier(notifier)

	if err := client.Watch(); err != nil {
		log.Fatalf("Couldn't watch Kubernetes API server: %s", err)
	}
}
