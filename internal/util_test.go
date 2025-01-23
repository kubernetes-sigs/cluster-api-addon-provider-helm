/*
Copyright 2025 The Kubernetes Authors.

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

package internal

import (
	"testing"

	. "github.com/onsi/gomega"
)

var (
	testMap = map[string]string{
		"> 1.32":     "0.32",
		"1.31":       "0.31",
		"1.30.x":     "0.30",
		"1.29":       "0.29",
		"1.28":       "0.28",
		"1.27":       "0.27",
		"1.26":       "0.26",
		"1.25":       "0.25",
		"1.24":       "0.24",
		"1.23":       "0.23",
		"1.22":       "0.22",
		"1.21":       "0.21",
		"1.20":       "0.20",
		"1.19":       "0.19",
		"1.18":       "0.18",
		"1.17":       "0.10",
		"1.9 - 1.16": "0.4 - 0.9",
		"1.7 - 1.8":  "0.3",
	}
)

func TestResolveHelmChartVersion(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name              string
		kubernetesVersion string
		versionMap        map[string]string
		expectedVersion   string
		expectedError     string
	}{
		{
			name:              "match exact minor version",
			kubernetesVersion: "v1.31",
			versionMap:        testMap,
			expectedVersion:   "0.31",
			expectedError:     "",
		},
		{
			name:              "match wildcard patch version",
			kubernetesVersion: "v1.30.1",
			versionMap:        testMap,
			expectedVersion:   "0.30",
			expectedError:     "",
		},
		{
			name:              "match version in range",
			kubernetesVersion: "v1.10",
			versionMap:        testMap,
			expectedVersion:   "0.4 - 0.9",
			expectedError:     "",
		},
		{
			name:              "ensure range is inclusive",
			kubernetesVersion: "v1.16",
			versionMap:        testMap,
			expectedVersion:   "0.4 - 0.9",
			expectedError:     "",
		},
		// TODO: add more tests
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			version, err := ResolveHelmChartVersion(tc.kubernetesVersion, tc.versionMap)

			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(version).To(Equal(tc.expectedVersion))
			}
		})
	}
}
