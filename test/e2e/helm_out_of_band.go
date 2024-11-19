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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api/test/framework"
)

// HelmReleaseOutOfBandInput specifies the input for updating an existing Helm release out-of-band and verifying that the changes do not get reverted
// on the InstallOnce strategy.
type HelmReleaseOutOfBandInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	HelmChartProxy        *addonsv1alpha1.HelmChartProxy // Note: Only the Spec field is used.
	Values                []string                       // The values to apply to the Helm release out-of-band.
	WaitPeriod            time.Duration                  // The time to wait before verifying that the Helm release is unchanged.
}

// HelmReleaseOutOfBandSpec implements a test that verifies an existing Helm release will not be changed on the InstallOnce strategy when the HelmChartProxy
// changes or the cluster is unselected. It assumes that the Helm release was installed successfully prior and verifies that the Helm release is unchanged.
func HelmReleaseOutOfBandSpec(ctx context.Context, inputGetter func() HelmReleaseOutOfBandInput) {
	var (
		specName   = "helm-upgrade"
		input      HelmReleaseOutOfBandInput
		mgmtClient ctrlclient.Client
		err        error
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)

	By("creating a Kubernetes client to the management cluster")
	mgmtClient = input.BootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	// Get existing HCP from management Cluster
	existing := &addonsv1alpha1.HelmChartProxy{}
	key := types.NamespacedName{
		Namespace: input.HelmChartProxy.Namespace,
		Name:      input.HelmChartProxy.Name,
	}
	err = mgmtClient.Get(ctx, key, existing)
	Expect(err).NotTo(HaveOccurred())

	// We want to get the Helm release using WaitForHelmReleaseProxyReady and WaitForHelmReleaseDeployed
	// First, wait for initial HelmReleaseProxy to be ready
	hrpWaitInput := GetWaitForHelmReleaseProxyReadyInput(ctx, bootstrapClusterProxy, input.ClusterName, *input.HelmChartProxy, 1, specName)
	WaitForHelmReleaseProxyReady(ctx, hrpWaitInput, e2eConfig.GetIntervals(specName, "wait-helmreleaseproxy-ready")...)

	// Then, get workload Cluster proxy
	By("creating a clusterctl proxy to the workload cluster")
	workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())

	// Finally, wait for Helm release on workload cluster to have status = deployed
	existingReleaseWaitInput := GetWaitForHelmReleaseDeployedInput(ctx, workloadClusterProxy, hrpWaitInput.HelmReleaseProxy.Spec.ReleaseName, hrpWaitInput.HelmReleaseProxy.Spec.ReleaseNamespace, specName)
	existingRelease := WaitForHelmReleaseDeployed(ctx, existingReleaseWaitInput, e2eConfig.GetIntervals(specName, "wait-helm-existingRelease-deployed")...)

	// Invoke UpgradeHelmChart logic here
	options := &HelmOptions{
		Values: input.Values,
	}
	UpgradeHelmChart(ctx, workloadClusterProxy, existingRelease.Namespace, input.HelmChartProxy.Spec.RepoURL, existingRelease.Chart.Name(), existingRelease.Name, options, existingRelease.Chart.Metadata.Version)
	// Is there a better way to ensure that the release won't change, besides waiting for a period of time and checking back?

	// Wait for Helm release on workload cluster to have status = deployed
	releaseWaitInput := GetWaitForHelmReleaseDeployedInput(ctx, workloadClusterProxy, hrpWaitInput.HelmReleaseProxy.Spec.ReleaseName, hrpWaitInput.HelmReleaseProxy.Spec.ReleaseNamespace, specName)
	release := WaitForHelmReleaseDeployed(ctx, releaseWaitInput, e2eConfig.GetIntervals(specName, "wait-helm-release-deployed")...)

	Byf("waiting for %v to ensure the Helm release does not change", input.WaitPeriod.Minutes())
	time.Sleep(input.WaitPeriod)

	EnsureHelmReleaseUnchanged(ctx, specName, input.BootstrapClusterProxy, input.ClusterName, input.Namespace.Name, release)
}
