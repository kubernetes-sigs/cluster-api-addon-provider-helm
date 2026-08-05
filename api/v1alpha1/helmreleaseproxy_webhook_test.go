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
	"context"
	"testing"

	. "github.com/onsi/gomega"
)

func TestHelmReleaseProxyValidateCreateRepoURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		repoURL   string
		expectErr bool
	}{
		{name: "https URL is allowed", repoURL: "https://charts.example.com/stable"},
		{name: "oci URL is allowed", repoURL: "oci://registry.example.com/charts"},
		{name: "http URL is rejected", repoURL: "http://charts.example.com", expectErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			proxy := &HelmReleaseProxy{Spec: HelmReleaseProxySpec{RepoURL: tc.repoURL}}

			_, err := (&helmReleaseProxyWebhook{}).ValidateCreate(context.Background(), proxy)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
