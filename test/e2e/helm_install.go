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
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api/test/framework"
)

// HelmInstallInput specifies the input for installing a Helm chart on a workload cluster and verifying that it was successful.
type HelmInstallInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	HelmChartProxy        *addonsv1alpha1.HelmChartProxy
}

// HelmInstallSpec implements a test that verifies a Helm chart can be installed on a workload cluster. It creates a HelmChartProxy
// resource and patches the Cluster labels such that they match the HelmChartProxy's clusterSelector. It then waits for the Helm
// release to be deployed on the workload cluster.
func HelmInstallSpec(ctx context.Context, inputGetter func() HelmInstallInput) {
	var (
		specName             = "helm-install"
		input                HelmInstallInput
		workloadClusterProxy framework.ClusterProxy
		mgmtClient           ctrlclient.Client
		err                  error
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)

	By("creating a Kubernetes client to the management cluster")
	mgmtClient = input.BootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	// Create HCP on management Cluster
	Byf("Creating HelmChartProxy %s/%s", input.HelmChartProxy.Namespace, input.HelmChartProxy.Name)
	Expect(mgmtClient.Create(ctx, input.HelmChartProxy)).To(Succeed())

	// Get Cluster from management Cluster
	workloadCluster := &clusterv1.Cluster{}
	key := types.NamespacedName{
		Namespace: input.Namespace.Name,
		Name:      input.ClusterName,
	}
	err = mgmtClient.Get(ctx, key, workloadCluster)
	Expect(err).NotTo(HaveOccurred())

	// Patch cluster labels, ignore match expressions for now
	selector := input.HelmChartProxy.Spec.ClusterSelector
	labels := workloadCluster.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	for k, v := range selector.MatchLabels {
		labels[k] = v
	}

	err = mgmtClient.Update(ctx, workloadCluster)
	Expect(err).NotTo(HaveOccurred())

	// Wait for HelmReleaseProxy to be ready
	hrpWaitInput := GetWaitForHelmReleaseProxyReadyInput(ctx, bootstrapClusterProxy, input.ClusterName, *input.HelmChartProxy, specName)
	WaitForHelmReleaseProxyReady(ctx, hrpWaitInput, e2eConfig.GetIntervals(specName, "wait-helmreleaseproxy-ready")...)

	// Get workload Cluster proxy
	By("creating a clusterctl proxy to the workload cluster")
	workloadClusterProxy = input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())

	// Wait for Helm release on workload cluster to have stauts = deployed
	releaseWaitInput := GetWaitForHelmReleaseDeployedInput(ctx, workloadClusterProxy, input.HelmChartProxy.Spec.ReleaseName, input.Namespace.Name, specName)
	WaitForHelmReleaseDeployed(ctx, releaseWaitInput, e2eConfig.GetIntervals(specName, "wait-helm-release-deployed")...)
}
