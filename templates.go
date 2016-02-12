package main

import (
	"os"
	"text/template"
)

type ServiceInformation struct {
	ServiceName string
	Port        int
	NodePort    int
}

type ClusterInformation struct {
	Services []ServiceInformation
	Nodes    []string
}

type Template struct {
	Source, Path string
}

func NewTemplate(source, path string) *Template {
	return &Template{
		Source: source,
		Path:   path,
	}
}

func (t *Template) Execute(info *ClusterInformation) error {
	s, err := template.ParseFiles(t.Source)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(t.Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = s.Execute(f, info); err != nil {
		return err
	}
	return nil
}
