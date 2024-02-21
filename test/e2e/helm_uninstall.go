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
	"github.com/pkg/errors"
	helmAction "helm.sh/helm/v3/pkg/action"
	helmDriver "helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
)

// HelmUninstallInput specifies the input for uninstalling a Helm chart on a workload cluster and verifying that it was successful.
type HelmUninstallInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	HelmChartProxy        *addonsv1alpha1.HelmChartProxy
}

// HelmUninstallSpec implements a test to uninstall a Helm chart from a given Cluster by removing the label selector from the Cluster.
func HelmUninstallSpec(ctx context.Context, inputGetter func() HelmUninstallInput) {
	var (
		specName   = "helm-uninstall"
		input      HelmUninstallInput
		mgmtClient ctrlclient.Client
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)

	By("creating a Kubernetes client to the management cluster")
	mgmtClient = input.BootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	// Get Cluster from management Cluster
	workloadCluster := &clusterv1.Cluster{}
	key := apitypes.NamespacedName{
		Namespace: input.Namespace.Name,
		Name:      input.ClusterName,
	}
	Expect(mgmtClient.Get(ctx, key, workloadCluster)).To(Succeed())

	// Patch cluster labels if not nil, ignore match expressions for now.
	// Ignore the case where the label selector matches all, since there isn't a way to unmatch a single cluster without unmatching all of them.
	selector := input.HelmChartProxy.Spec.ClusterSelector
	if workloadCluster.Labels != nil {
		hrp, err := getHelmReleaseProxy(ctx, mgmtClient, input.ClusterName, *input.HelmChartProxy)
		Expect(err).NotTo(HaveOccurred())

		for k := range selector.MatchLabels {
			delete(workloadCluster.Labels, k)
		}

		Expect(mgmtClient.Update(ctx, workloadCluster)).To(Succeed())

		// Verify that the HelmReleaseProxy is deleted as well as the Helm release.
		// We want to fetch the Helm release first, then, so we can check the release name field.
		Eventually(func() error {
			key := apitypes.NamespacedName{
				Namespace: hrp.Namespace,
				Name:      hrp.Name,
			}
			if err := mgmtClient.Get(ctx, key, &addonsv1alpha1.HelmReleaseProxy{}); apierrors.IsNotFound(err) {
				return nil
			}

			return errors.Errorf("HelmReleaseProxy %s still exists", key.Name)
		}, e2eConfig.GetIntervals(specName, "wait-delete-helmreleaseproxy")...).Should(Succeed())

		// Get workload Cluster proxy
		By("creating a clusterctl proxy to the workload cluster")
		workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
		Expect(workloadClusterProxy).NotTo(BeNil())
		actionConfig := getHelmActionConfigForTests(ctx, workloadClusterProxy, hrp.Spec.ReleaseNamespace)

		// Wait for Helm release to be deleted
		releaseName := hrp.Spec.ReleaseName
		Eventually(func() error {
			getClient := helmAction.NewGet(actionConfig)
			if r, err := getClient.Run(releaseName); err != nil {
				if err == helmDriver.ErrReleaseNotFound {
					return nil
				}
				return err
			} else {
				return errors.Errorf("Helm release %s still exists", r.Name)
			}
		}, e2eConfig.GetIntervals(specName, "wait-helm-release")...).Should(Succeed())
	}

}
