/*
Copyright 2019 The Kubernetes Authors.

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

package cloudinit

import (
	"encoding/json"

	"github.com/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
)

type unknown struct {
	module string
	lines  []string
}

func newUnknown(module string) action {
	return &unknown{module: module}
}

// Unmarshal will unmarshall unknown actions and slurp the value
func (u *unknown) Unmarshal(data []byte) error {
	// try unmarshalling to a slice of strings
	var s1 []string
	if err := json.Unmarshal(data, &s1); err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); !ok {
			return errors.WithStack(err)
		}
	} else {
		u.lines = s1
		return nil
	}

	// If it's not a slice of strings it should be one string value
	var s2 string
	if err := json.Unmarshal(data, &s2); err != nil {
		return errors.WithStack(err)
	}

	u.lines = []string{s2}
	return nil
}

// Run will do nothing since the cloud config module is unknown.
func (u *unknown) Run(_ exec.Cmder) ([]string, error) {
	return u.lines, nil
}
