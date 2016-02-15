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

import "testing"

var definitionCases = []struct {
	Definition string
	Error      bool
}{
	{"notexists:foo", true},
	{"", true},
	{"::", true},
	{"debug", true},
	{"debug:", false},
	{"pid::100", true},
	{"pid:SIGTERM:100", false},
	{"pidfile:SIGTERM:test.pid", false},
}

func TestNotifierDefinitions(t *testing.T) {
	for _, d := range definitionCases {
		n, err := NewNotifier(d.Definition)
		if (err != nil) != d.Error {
			t.Logf("Definition: %v, Expected error? %v\n", d.Definition, d.Error)
			t.Logf("Notifier: %v\n", n)
			t.Logf("Error: %v\n", err)
			t.Fail()
		}
	}
}
