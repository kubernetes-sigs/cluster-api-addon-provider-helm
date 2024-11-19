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
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api/test/framework"
)

// HelmReleaseUnchangedInput specifies the input for updating or reinstalling a Helm chart on a workload cluster and verifying that it was successful.
type HelmReleaseUnchangedInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	HelmChartProxy        *addonsv1alpha1.HelmChartProxy // Note: Only the Spec field is used.
}

// HelmReleaseUnchangedSpec implements a test that verifies an existing Helm release will not be changed on the InstallOnce strategy when the HelmChartProxy
// changes or the cluster is unselected. It assumes that the Helm release was installed successfully prior and verifies that the Helm release is unchanged.
func HelmReleaseUnchangedSpec(ctx context.Context, inputGetter func() HelmReleaseUnchangedInput) {
	var (
		specName   = "helm-upgrade"
		input      HelmReleaseUnchangedInput
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
	releaseWaitInput := GetWaitForHelmReleaseDeployedInput(ctx, workloadClusterProxy, hrpWaitInput.HelmReleaseProxy.Spec.ReleaseName, hrpWaitInput.HelmReleaseProxy.Spec.ReleaseNamespace, specName)
	release := WaitForHelmReleaseDeployed(ctx, releaseWaitInput, e2eConfig.GetIntervals(specName, "wait-helm-release-deployed")...)

	// Patch HCP on management Cluster
	Byf("Patching HelmChartProxy %s/%s", existing.Namespace, existing.Name)
	patchHelper, err := patch.NewHelper(existing, mgmtClient)
	Expect(err).ToNot(HaveOccurred())

	existing.Spec = input.HelmChartProxy.Spec
	input.HelmChartProxy = existing

	Eventually(func() error {
		return patchHelper.Patch(ctx, existing)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to patch HelmChartProxy %s", klog.KObj(existing))

	EnsureHelmReleaseUnchanged(ctx, specName, input.BootstrapClusterProxy, input.ClusterName, input.Namespace.Name, release)
}
