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
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	helmAction "helm.sh/helm/v3/pkg/action"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
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

func TestGenerateHelmInstallConfig(t *testing.T) {
	t.Parallel()

	timeout := &metav1.Duration{Duration: 5 * time.Minute}

	testCases := []struct {
		name        string
		helmOptions *addonsv1alpha1.HelmOptions
		assert      func(*GomegaWithT, *helmAction.Install)
	}{
		{
			name:        "nil helmOptions returns defaults",
			helmOptions: nil,
			assert: func(g *GomegaWithT, install *helmAction.Install) {
				g.Expect(install.CreateNamespace).To(BeTrue())
				g.Expect(install.TakeOwnership).To(BeFalse())
				g.Expect(install.DisableHooks).To(BeFalse())
				g.Expect(install.Wait).To(BeFalse())
			},
		},
		{
			name:        "empty helmOptions",
			helmOptions: &addonsv1alpha1.HelmOptions{},
			assert: func(g *GomegaWithT, install *helmAction.Install) {
				g.Expect(install.TakeOwnership).To(BeFalse())
				g.Expect(install.CreateNamespace).To(BeFalse())
				g.Expect(install.DisableHooks).To(BeFalse())
			},
		},
		{
			name: "TakeOwnership set to true",
			helmOptions: &addonsv1alpha1.HelmOptions{
				TakeOwnership: true,
			},
			assert: func(g *GomegaWithT, install *helmAction.Install) {
				g.Expect(install.TakeOwnership).To(BeTrue())
			},
		},
		{
			name: "all options set",
			helmOptions: &addonsv1alpha1.HelmOptions{
				DisableHooks:             true,
				Wait:                     true,
				WaitForJobs:              true,
				Timeout:                  timeout,
				SkipCRDs:                 true,
				SubNotes:                 true,
				DisableOpenAPIValidation: true,
				Atomic:                   true,
				TakeOwnership:            true,
				Install: addonsv1alpha1.HelmInstallOptions{
					CreateNamespace: true,
					IncludeCRDs:     true,
				},
			},
			assert: func(g *GomegaWithT, install *helmAction.Install) {
				g.Expect(install.DisableHooks).To(BeTrue())
				g.Expect(install.Wait).To(BeTrue())
				g.Expect(install.WaitForJobs).To(BeTrue())
				g.Expect(install.Timeout).To(Equal(5 * time.Minute))
				g.Expect(install.SkipCRDs).To(BeTrue())
				g.Expect(install.SubNotes).To(BeTrue())
				g.Expect(install.DisableOpenAPIValidation).To(BeTrue())
				g.Expect(install.Atomic).To(BeTrue())
				g.Expect(install.TakeOwnership).To(BeTrue())
				g.Expect(install.CreateNamespace).To(BeTrue())
				g.Expect(install.IncludeCRDs).To(BeTrue())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			actionConfig := &helmAction.Configuration{}
			install := generateHelmInstallConfig(actionConfig, tc.helmOptions)
			g.Expect(install).ToNot(BeNil())
			tc.assert(g, install)
		})
	}
}

func TestGenerateHelmUpgradeConfig(t *testing.T) {
	t.Parallel()

	timeout := &metav1.Duration{Duration: 5 * time.Minute}

	testCases := []struct {
		name        string
		helmOptions *addonsv1alpha1.HelmOptions
		assert      func(*GomegaWithT, *helmAction.Upgrade)
	}{
		{
			name:        "nil helmOptions returns defaults",
			helmOptions: nil,
			assert: func(g *GomegaWithT, upgrade *helmAction.Upgrade) {
				g.Expect(upgrade.TakeOwnership).To(BeFalse())
				g.Expect(upgrade.DisableHooks).To(BeFalse())
				g.Expect(upgrade.Wait).To(BeFalse())
			},
		},
		{
			name:        "empty helmOptions",
			helmOptions: &addonsv1alpha1.HelmOptions{},
			assert: func(g *GomegaWithT, upgrade *helmAction.Upgrade) {
				g.Expect(upgrade.TakeOwnership).To(BeFalse())
				g.Expect(upgrade.DisableHooks).To(BeFalse())
			},
		},
		{
			name: "TakeOwnership set to true",
			helmOptions: &addonsv1alpha1.HelmOptions{
				TakeOwnership: true,
			},
			assert: func(g *GomegaWithT, upgrade *helmAction.Upgrade) {
				g.Expect(upgrade.TakeOwnership).To(BeTrue())
			},
		},
		{
			name: "all options set",
			helmOptions: &addonsv1alpha1.HelmOptions{
				DisableHooks:             true,
				Wait:                     true,
				WaitForJobs:              true,
				Timeout:                  timeout,
				SkipCRDs:                 true,
				SubNotes:                 true,
				DisableOpenAPIValidation: true,
				Atomic:                   true,
				TakeOwnership:            true,
				Upgrade: addonsv1alpha1.HelmUpgradeOptions{
					Force:                true,
					ResetValues:          true,
					ReuseValues:          true,
					ResetThenReuseValues: true,
					MaxHistory:           10,
					CleanupOnFail:        true,
				},
			},
			assert: func(g *GomegaWithT, upgrade *helmAction.Upgrade) {
				g.Expect(upgrade.DisableHooks).To(BeTrue())
				g.Expect(upgrade.Wait).To(BeTrue())
				g.Expect(upgrade.WaitForJobs).To(BeTrue())
				g.Expect(upgrade.Timeout).To(Equal(5 * time.Minute))
				g.Expect(upgrade.SkipCRDs).To(BeTrue())
				g.Expect(upgrade.SubNotes).To(BeTrue())
				g.Expect(upgrade.DisableOpenAPIValidation).To(BeTrue())
				g.Expect(upgrade.Atomic).To(BeTrue())
				g.Expect(upgrade.TakeOwnership).To(BeTrue())
				g.Expect(upgrade.Force).To(BeTrue())
				g.Expect(upgrade.ResetValues).To(BeTrue())
				g.Expect(upgrade.ReuseValues).To(BeTrue())
				g.Expect(upgrade.ResetThenReuseValues).To(BeTrue())
				g.Expect(upgrade.MaxHistory).To(Equal(10))
				g.Expect(upgrade.CleanupOnFail).To(BeTrue())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			actionConfig := &helmAction.Configuration{}
			upgrade := generateHelmUpgradeConfig(actionConfig, tc.helmOptions)
			g.Expect(upgrade).ToNot(BeNil())
			tc.assert(g, upgrade)
		})
	}
}
