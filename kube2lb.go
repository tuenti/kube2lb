package main

import (
	"flag"
	"log"
)

func main() {
	var apiserver, config, template, notify string
	flag.StringVar(&apiserver, "apiserver", "http://localhost:8080", "Kubernetes API server URL")
	flag.StringVar(&config, "config", "", "Configuration path to generate")
	flag.StringVar(&template, "template", "", "Configuration source template")
	flag.StringVar(&notify, "notify", "", "Kubernetes API server URL")
	flag.Parse()

	client, err := NewKubernetesClient(apiserver)
	if err != nil {
		log.Fatalf("Couldn't connect with Kubernetes API server: %s", err)
	}

	notifier, err := NewNotifier(notify)
	if err != nil {
		log.Fatalf("Couldn't initialize notifier: %s", err)
	}

	client.AddTemplate(NewTemplate(template, config))
	client.AddNotifier(notifier)
	client.Update()
	client.Watch()
}
