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

// HelmUpgradeInput specifies the input for updating or reinstalling a Helm chart on a workload cluster and verifying that it was successful.
type HelmUpgradeInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	HelmChartProxy        *addonsv1alpha1.HelmChartProxy // Note: Only the Spec field is used.
	ExpectedRevision      int
}

// HelmUpgradeSpec implements a test that verifies a Helm chart can be either updated or reinstalled on a workload cluster, depending on
// if an immutable field has changed. It takes a HelmChartProxy resource and updates ONLY the spec field and patches the Cluster labels
// such that they match the HelmChartProxy's clusterSelector. It then waits for the Helm release to be deployed on the workload cluster
// and validates the release and the HelmReleaseProxy status fields.
func HelmUpgradeSpec(ctx context.Context, inputGetter func() HelmUpgradeInput) {
	var (
		specName   = "helm-upgrade"
		input      HelmUpgradeInput
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

	// Patch HCP on management Cluster
	Byf("Patching HelmChartProxy %s/%s", existing.Namespace, existing.Name)
	patchHelper, err := patch.NewHelper(existing, mgmtClient)
	Expect(err).ToNot(HaveOccurred())

	existing.Spec = input.HelmChartProxy.Spec
	input.HelmChartProxy = existing

	Eventually(func() error {
		return patchHelper.Patch(ctx, existing)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to patch HelmChartProxy %s", klog.KObj(existing))

	EnsureHelmReleaseInstallOrUpgrade(ctx, specName, input.BootstrapClusterProxy, nil, &input, true)
}
