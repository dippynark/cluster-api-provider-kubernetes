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

package framework_test

import (
	"testing"

	"github.com/dippynark/cluster-api-provider-kubernetes/e2e/framework"
)

func TestTypeToKind(t *testing.T) {
	type hello struct{}

	out := framework.TypeToKind(&hello{})
	if out != "hello" {
		t.Fatalf("Expected %q from pointer input but got %q", "hello", out)
	}
}
