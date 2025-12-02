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
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"helm.sh/helm/v3/pkg/chart"
	helmRelease "helm.sh/helm/v3/pkg/release"
	"k8s.io/klog/v2/ktesting"
	_ "k8s.io/klog/v2/ktesting/init"
)

func TestNewDefaultRegistryClient(t *testing.T) {
	t.Parallel()

	emptyFilepath := filepath.Join(t.TempDir(), "empty-file")
	NewWithT(t).Expect(os.Create(emptyFilepath)).Error().ShouldNot(HaveOccurred())

	testCases := []struct {
		name                  string
		credentialsPath       string
		enableCache           bool
		caFilePath            string
		insecureSkipTLSVerify bool
		assertErr             types.GomegaMatcher
		assertClient          types.GomegaMatcher
	}{
		{
			name:                  "default config",
			credentialsPath:       "",
			enableCache:           true,
			caFilePath:            "",
			insecureSkipTLSVerify: false,
			assertErr:             Not(HaveOccurred()),
			assertClient:          Not(BeNil()),
		},
		{
			name:                  "skip tls verification",
			credentialsPath:       "",
			enableCache:           true,
			caFilePath:            "",
			insecureSkipTLSVerify: true,
			assertErr:             Not(HaveOccurred()),
			assertClient:          Not(BeNil()),
		},
		{
			name:                  "invalid ca bundle path",
			credentialsPath:       "",
			enableCache:           true,
			caFilePath:            filepath.Join(t.TempDir(), "non-existing"),
			insecureSkipTLSVerify: false,
			assertErr:             MatchError(ContainSubstring("can't create TLS config")),
			assertClient:          BeNil(),
		},
		{
			name:                  "empty ca bundle file",
			credentialsPath:       "",
			enableCache:           true,
			caFilePath:            emptyFilepath,
			insecureSkipTLSVerify: false,
			assertErr:             MatchError(ContainSubstring("failed to append certificates from file")),
			assertClient:          BeNil(),
		},
		{
			name:                  "valid ca bundle",
			credentialsPath:       "",
			enableCache:           true,
			caFilePath:            "testdata/ca-bundle.pem",
			insecureSkipTLSVerify: false,
			assertErr:             Not(HaveOccurred()),
			assertClient:          Not(BeNil()),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			client, err := newDefaultRegistryClient(
				tc.credentialsPath,
				tc.enableCache,
				tc.caFilePath,
				tc.insecureSkipTLSVerify,
			)
			g.Expect(err).To(tc.assertErr)
			g.Expect(client).To(tc.assertClient)
		})
	}
}

func TestShouldUpgradeHelmRelease(t *testing.T) {
	deployed := &helmRelease.Info{
		Status: helmRelease.StatusDeployed,
	}
	pending := &helmRelease.Info{
		Status: helmRelease.StatusPendingInstall,
	}
	failed := &helmRelease.Info{
		Status: helmRelease.StatusFailed,
	}

	chartV0 := &chart.Chart{
		Metadata: &chart.Metadata{
			Version: "0.0.0",
		},
	}
	chartV1 := &chart.Chart{
		Metadata: &chart.Metadata{
			Version: "1.0.0",
		},
	}

	tests := []struct {
		name           string
		existing       helmRelease.Release
		chartRequested *chart.Chart
		values         map[string]any
		expected       bool
		expectedErr    error
	}{
		{
			name:        "no existing chart",
			existing:    helmRelease.Release{Chart: nil},
			expectedErr: errNoChartVersion,
		},
		{
			name: "updated chart version",
			existing: helmRelease.Release{
				Chart: chartV0,
			},
			chartRequested: chartV1,
			expected:       true,
		},
		{
			name: "existing failed status",
			existing: helmRelease.Release{
				Chart: chartV0,
				Info:  failed,
			},
			chartRequested: chartV0,
			expected:       true,
		},
		{
			name: "existing pending status",
			existing: helmRelease.Release{
				Chart: chartV0,
				Info:  pending,
			},
			chartRequested: chartV0,
			expected:       true,
		},
		{
			name: "identical values",
			existing: helmRelease.Release{
				Chart: chartV0,
				Info:  deployed,
				Config: map[string]any{
					"key": float64(123),
				},
			},
			chartRequested: chartV0,
			values: map[string]any{
				"key": 123,
			},
			expected: false,
		},
		{
			name: "modified values",
			existing: helmRelease.Release{
				Chart: chartV0,
				Info:  deployed,
				Config: map[string]any{
					"key": float64(123),
				},
			},
			chartRequested: chartV0,
			values: map[string]any{
				"key": 456,
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			_, ctx := ktesting.NewTestContext(t)

			actual, err := shouldUpgradeHelmRelease(ctx, test.existing, test.chartRequested, test.values)
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(actual).To(Equal(test.expected))
		})
	}
}
