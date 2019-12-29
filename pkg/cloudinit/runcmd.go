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
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

// Cmd defines a runcmd command
type Cmd struct {
	Cmd  string
	Args []string
}

// UnmarshalJSON a runcmd command
// It can be either a list or a string. If the item is a
// list, it will be properly executed (with the first arg as the command).
// If the item is a string, it will be written to a file and interpreted using ``sh``.
func (c *Cmd) UnmarshalJSON(data []byte) error {
	// try to decode the command into a list
	var s1 []string
	if err := json.Unmarshal(data, &s1); err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); !ok {
			return errors.WithStack(err)
		}
	} else {
		c.Cmd = s1[0]
		c.Args = s1[1:]
		return nil
	}

	// if decode into a list didn't worked,
	// try to decode the command into a string
	var s2 string
	if err := json.Unmarshal(data, &s2); err != nil {
		return errors.WithStack(err)
	}

	c.Cmd = "/bin/sh"
	c.Args = []string{"-c", s2}

	return nil
}

// runCmd defines a cloud init action that replicates the behavior of the cloud init rundcmd module
type runCmd struct {
	Cmds []Cmd `json:"runcmd,"`
}

func newRunCmdAction() action {
	return &runCmd{}
}

// Unmarshal the runCmd
func (a *runCmd) Unmarshal(userData []byte) error {
	if err := yaml.Unmarshal(userData, a); err != nil {
		return errors.Wrapf(err, "error parsing run_cmd action: %s", userData)
	}
	return nil
}

// Generate the runCmd
func (a *runCmd) GenerateScriptBlock() (string, error) {
	scriptBlock := fmt.Sprintf("# %s script block", runcmd)
	for _, c := range a.Cmds {
		// kubeadm in docker requires to ignore some errors, and this requires to modify the cmd generate by CABPK by default...
		c = hackKubeadmIgnoreErrors(c)

		// Append line to block
		command := fmt.Sprintf("%q", c.Cmd)
		for _, arg := range c.Args {
			command = fmt.Sprintf("%s %q", command, arg)
		}
		scriptBlock = fmt.Sprintf("%s\n%s", scriptBlock, command)
	}

	return scriptBlock, nil
}

func hackKubeadmIgnoreErrors(c Cmd) Cmd {
	// case kubeadm commands are defined as a string
	if c.Cmd == "/bin/sh" && len(c.Args) >= 2 {
		if c.Args[0] == "-c" && (strings.Contains(c.Args[1], "kubeadm init") || strings.Contains(c.Args[1], "kubeadm join")) {
			c.Args[1] = fmt.Sprintf("%s %s", c.Args[1], "--ignore-preflight-errors=all")
		}
	}

	// case kubeadm commands are defined as a list
	if c.Cmd == "kubeadm" && len(c.Args) >= 1 {
		if c.Args[0] == "init" || c.Args[0] == "join" {
			c.Args = append(c.Args, "--ignore-preflight-errors=all")
		}
	}

	return c
}
