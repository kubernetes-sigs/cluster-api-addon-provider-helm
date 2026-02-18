//go:build e2e
// +build e2e

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

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	helmRelease "helm.sh/helm/v3/pkg/release"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// HelmPendingInput specifies the input for installing a Helm chart on a workload cluster and verifying that it was successful.
type HelmPendingInput struct {
	HelmInstallInput
	LastReleaseStatus helmRelease.Status
}

// HelmPendingSpec implements a test that verifies a Helm chart can be upgraded, or uninstalled on a workload cluster when the Helm release is in a pending status.
// It first installs the chart on the workload cluster.
// It then overwrites the last Helm release status to the provided status and waits for the controller to successfully upgrade or uninstall the chart.
func HelmPendingSpec(ctx context.Context, inputGetter func() HelmPendingInput) {
	var (
		specName   = "helm-pending"
		input      HelmPendingInput
		mgmtClient ctrlclient.Client
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.HelmChartProxy).NotTo(BeNil(), "Invalid argument. input.HelmChartProxy can't be nil when calling %s spec", specName)
	Expect(input.LastReleaseStatus).To(BeElementOf(
		helmRelease.StatusPendingInstall,
		helmRelease.StatusPendingUpgrade,
		helmRelease.StatusPendingRollback,
		helmRelease.StatusUninstalling,
	),
		"Invalid argument LastReleaseStatus %s. It must be one of %v when calling %s spec",
		input.LastReleaseStatus,
		[]helmRelease.Status{helmRelease.StatusPendingInstall, helmRelease.StatusPendingUpgrade, helmRelease.StatusPendingRollback, helmRelease.StatusUninstalling},
		specName,
	)

	By("creating a Kubernetes client to the management cluster")
	mgmtClient = input.BootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	By("Installing the chart", func() {
		HelmInstallSpec(ctx, func() HelmInstallInput {
			return HelmInstallInput{
				BootstrapClusterProxy: input.BootstrapClusterProxy,
				Namespace:             input.Namespace,
				ClusterName:           input.ClusterName,
				HelmChartProxy:        input.HelmChartProxy,
			}
		})
	})

	Byf("Overwriting the last Helm release status to %s", input.LastReleaseStatus)
	hrp, err := getHelmReleaseProxy(ctx, mgmtClient, input.ClusterName, *input.HelmChartProxy)
	Expect(err).NotTo(HaveOccurred())
	Expect(hrp).NotTo(BeNil())
	workloadClusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())
	SetHelmReleaseStatus(ctx, workloadClusterProxy, hrp.Spec.ReleaseNamespace, hrp.Spec.ReleaseName, input.LastReleaseStatus)

	switch input.LastReleaseStatus {
	case helmRelease.StatusPendingInstall, helmRelease.StatusPendingUpgrade, helmRelease.StatusPendingRollback:
		By("Waiting for controller to successfully upgrade the chart")
		EnsureHelmReleaseInstallOrUpgrade(
			ctx,
			specName,
			bootstrapClusterProxy,
			nil, // installInput
			&HelmUpgradeInput{
				BootstrapClusterProxy: input.BootstrapClusterProxy,
				Namespace:             input.Namespace,
				ClusterName:           input.ClusterName,
				HelmChartProxy:        input.HelmChartProxy,
				ExpectedRevision:      2, // Expected revision is 2 because the controller will upgrade the chart.
			},
			false,
		)
	case helmRelease.StatusUninstalling:
		By("Waiting for controller to successfully uninstall the chart")
		HelmUninstallSpec(ctx, func() HelmUninstallInput {
			return HelmUninstallInput{
				BootstrapClusterProxy: input.BootstrapClusterProxy,
				Namespace:             input.Namespace,
				ClusterName:           input.ClusterName,
				HelmChartProxy:        input.HelmChartProxy,
			}
		})
	}
}
