/*
Copyright 2026 The Kubernetes Authors.

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

package v1alpha1

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestRepoURLValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		repoURL   string
		expectErr bool
	}{
		{name: "https is allowed", repoURL: "https://charts.example.com/stable"},
		{name: "oci is allowed", repoURL: "oci://registry.example.com/charts"},
		{name: "uppercase scheme is allowed", repoURL: "HTTPS://charts.example.com"},
		{name: "http is rejected", repoURL: "http://charts.example.com", expectErr: true},
		{name: "ftp is rejected", repoURL: "ftp://charts.example.com", expectErr: true},
		{name: "file is rejected", repoURL: "file:///etc/passwd", expectErr: true},
		{name: "bare path is rejected", repoURL: "/some/local/path", expectErr: true},
		{name: "missing host is rejected", repoURL: "https:///charts", expectErr: true},
		{name: "empty URL is rejected", repoURL: "", expectErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			err := isUrlValid(tc.repoURL)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
