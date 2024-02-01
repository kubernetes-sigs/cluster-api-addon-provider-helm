//go:build e2e
// +build e2e

/*
Copyright 2024 The Kubernetes Authors.

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

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var _ = Describe("Running the Cluster API E2E tests", func() {

	AfterEach(func() {
		CheckTestBeforeCleanup()
	})

	Context("Running the quick-start spec [PR-Blocking]", func() {
		capi_e2e.QuickStartSpec(context.TODO(), func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				Flavor:                ptr.To(""),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}
		})
	})

	// TODO: figure out if we can run this locally.
	// if os.Getenv("USE_LOCAL_KIND_REGISTRY") != "true" {
	Context("API Version Upgrade", func() {

		Context("upgrade from an old version of v1beta1 to current, and scale workload clusters created in the old version", func() {

			capi_e2e.ClusterctlUpgradeSpec(context.TODO(), func() capi_e2e.ClusterctlUpgradeSpecInput {
				return capi_e2e.ClusterctlUpgradeSpecInput{
					E2EConfig:             e2eConfig,
					ClusterctlConfigPath:  clusterctlConfigPath,
					BootstrapClusterProxy: bootstrapClusterProxy,
					ArtifactFolder:        artifactFolder,
					SkipCleanup:           skipCleanup,
					// PreInit:                   getPreInitFunc(ctx),
					InitWithProvidersContract: "v1beta1",
					ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
						WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
					},
					InitWithKubernetesVersion:       "v1.27.3",
					InitWithBinary:                  "https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.5.0/clusterctl-{OS}-{ARCH}",
					InitWithCoreProvider:            "cluster-api:v1.5.0",
					InitWithBootstrapProviders:      []string{"kubeadm:v1.5.0"},
					InitWithControlPlaneProviders:   []string{"kubeadm:v1.5.0"},
					InitWithInfrastructureProviders: []string{"docker:v1.5.0"},
					InitWithAddonProviders:          []string{"helm:v0.1.1-alpha.0"},
				}
			})
		})
	})
	// }

})
