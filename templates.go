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
	"bytes"
	"flag"
	"os"
	"path"
	"strings"
	"text/template"
)

var defaultServerNameTemplate = "{{ .Service.Name }}.{{ .Service.Namespace }}.svc.{{ .Domain }}"
var serverNameTemplatesArg string
var serverNameTemplates []*template.Template

func init() {
	flag.StringVar(&serverNameTemplatesArg, "server-name-templates", defaultServerNameTemplate, "Comma-separated list of go templates to generate server names")
}

func parseServerNameTemplatesArg(templatesArg string) ([]*template.Template, error) {
	if len(templatesArg) == 0 {
		templatesArg = defaultServerNameTemplate
	}
	templateStrings := strings.Split(templatesArg, ",")
	templates := make([]*template.Template, len(templateStrings))
	for i, templateString := range templateStrings {
		t, err := template.New("server_name").Parse(templateString)
		if err != nil {
			return nil, err
		}
		templates[i] = t
	}
	return templates, nil
}

func initServerNameTemplates() (err error) {
	if len(serverNameTemplates) > 0 {
		return nil
	}
	serverNameTemplates, err = parseServerNameTemplatesArg(serverNameTemplatesArg)
	return err
}

type ServiceInformation struct {
	Name      string
	Namespace string
	Port      int
	NodePort  int
	External  []string
}

type ClusterInformation struct {
	Services []ServiceInformation
	Ports    []int
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

func removeDuplicated(names []string) []string {
	seen := make(map[string]interface{})
	for _, name := range names {
		seen[name] = nil
	}
	uniq := make([]string, 0, len(seen))
	for k := range seen {
		uniq = append(uniq, k)
	}
	return uniq
}

func generateServerNames(s ServiceInformation, domain string) []string {
	serverNames := make([]string, len(serverNameTemplates))
	for i, t := range serverNameTemplates {
		data := struct {
			Service ServiceInformation
			Domain  string
		}{s, domain}
		var serverName bytes.Buffer
		t.Execute(&serverName, data)
		serverNames[i] = serverName.String()
	}
	return removeDuplicated(append(serverNames, s.External...))
}

var nodeNameReplacer = strings.NewReplacer(".", "_")

func (t *Template) Execute(info *ClusterInformation) error {
	funcMap := template.FuncMap{
		"ServerNames": generateServerNames,
		"EscapeNode":  nodeNameReplacer.Replace,
	}

	// template.Execute will use the base name of t.Source
	s, err := template.New(path.Base(t.Source)).Funcs(funcMap).ParseFiles(t.Source)
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
