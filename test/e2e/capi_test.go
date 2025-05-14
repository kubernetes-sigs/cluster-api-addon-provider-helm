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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"k8s.io/utils/ptr"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var _ = Describe("Running the Cluster API E2E tests", func() {

	AfterEach(func() {
		CheckTestBeforeCleanup()
	})

	Context("Running the quick-start spec [PR-Blocking]", func() {
		capie2e.QuickStartSpec(context.TODO(), func() capie2e.QuickStartSpecInput {
			return capie2e.QuickStartSpecInput{
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

	Context("Running the workload cluster K8s version upgrade spec [K8s-Upgrade]", func() {
		capie2e.ClusterUpgradeConformanceSpec(context.TODO(), func() capie2e.ClusterUpgradeConformanceSpecInput {
			return capie2e.ClusterUpgradeConformanceSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				SkipConformanceTests:  true,
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}
		})
	})

	Context("API Version Upgrade", func() {

		Context("upgrade from an old version of v1beta1 to current, and scale workload clusters created in the old version", func() {

			capie2e.ClusterctlUpgradeSpec(context.TODO(), func() capie2e.ClusterctlUpgradeSpecInput {
				return capie2e.ClusterctlUpgradeSpecInput{
					E2EConfig:                 e2eConfig,
					ClusterctlConfigPath:      clusterctlConfigPath,
					BootstrapClusterProxy:     bootstrapClusterProxy,
					ArtifactFolder:            artifactFolder,
					SkipCleanup:               skipCleanup,
					InitWithProvidersContract: "v1beta1",
					ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
						WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
					},
					InitWithKubernetesVersion:       e2eConfig.GetVariable(KubernetesVersionAPIUpgradeFrom),
					InitWithBinary:                  fmt.Sprintf("https://github.com/kubernetes-sigs/cluster-api/releases/download/%s/clusterctl-{OS}-{ARCH}", e2eConfig.GetVariable(OldCAPIUpgradeVersion)),
					InitWithCoreProvider:            "cluster-api:" + e2eConfig.GetVariable(OldCAPIUpgradeVersion),
					InitWithBootstrapProviders:      []string{"kubeadm:" + e2eConfig.GetVariable(OldCAPIUpgradeVersion)},
					InitWithControlPlaneProviders:   []string{"kubeadm:" + e2eConfig.GetVariable(OldCAPIUpgradeVersion)},
					InitWithInfrastructureProviders: []string{"docker:" + e2eConfig.GetVariable(OldCAPIUpgradeVersion)},
					InitWithAddonProviders:          []string{"helm:" + e2eConfig.GetVariable(OldProviderUpgradeVersion)},
				}
			})
		})
	})
})
