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
	"os"
	"text/template"
)

type ServiceInformation struct {
	Name      string
	Namespace string
	Port      int
	NodePort  int
}

type ClusterInformation struct {
	Services []ServiceInformation
	Nodes    []string
	Domain   string
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
