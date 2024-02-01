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
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kubeadmv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubesystem = "kube-system"
)

// EnsureControlPlaneInitialized waits for the cluster KubeadmControlPlane object to be initialized
// and then installs cloud-provider-azure components via Helm.
// Fulfills the clusterctl.Waiter type so that it can be used as ApplyClusterTemplateAndWaitInput data
// in the flow of a clusterctl.ApplyClusterTemplateAndWait E2E test scenario.
func EnsureControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
	getter := input.ClusterProxy.GetClient()
	cluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
		Getter:    getter,
		Name:      input.ClusterName,
		Namespace: input.Namespace,
	})
	kubeadmControlPlane := &kubeadmv1.KubeadmControlPlane{}
	key := client.ObjectKey{
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}

	By("Ensuring KubeadmControlPlane is initialized")
	Eventually(func(g Gomega) {
		g.Expect(getter.Get(ctx, key, kubeadmControlPlane)).To(Succeed(), "Failed to get KubeadmControlPlane object %s/%s", cluster.Spec.ControlPlaneRef.Namespace, cluster.Spec.ControlPlaneRef.Name)
		g.Expect(kubeadmControlPlane.Status.Initialized).To(BeTrue(), "KubeadmControlPlane is not yet initialized")
	}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "KubeadmControlPlane object %s/%s was not initialized in time", cluster.Spec.ControlPlaneRef.Namespace, cluster.Spec.ControlPlaneRef.Name)

	By("Ensuring API Server is reachable before querying Helm charts")
	Eventually(func(g Gomega) {
		ns := &corev1.Namespace{}
		clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
		g.Expect(clusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: kubesystem}, ns)).To(Succeed(), "Failed to get kube-system namespace")
	}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "API Server was not reachable in time")

	By("Ensure calico is ready after control plane is initialized")
	EnsureCalicoIsReady(ctx, input)

	result.ControlPlane = framework.DiscoveryAndWaitForControlPlaneInitialized(ctx, framework.DiscoveryAndWaitForControlPlaneInitializedInput{
		Lister:  input.ClusterProxy.GetClient(),
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

const (
	calicoHelmChartRepoURL   string = "https://docs.tigera.io/calico/charts"
	calicoOperatorNamespace  string = "tigera-operator"
	CalicoSystemNamespace    string = "calico-system"
	CalicoAPIServerNamespace string = "calico-apiserver"
	calicoHelmReleaseName    string = "projectcalico"
	calicoHelmChartName      string = "tigera-operator"
)

// EnsureCalicoIsReady verifies that the calico deployments exist and and are available on the workload cluster.
func EnsureCalicoIsReady(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput) {
	specName := "ensure-calico"

	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)

	By("Waiting for Ready tigera-operator deployment pods")
	for _, d := range []string{"tigera-operator"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, calicoOperatorNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}

	By("Waiting for Ready calico-system deployment pods")
	for _, d := range []string{"calico-kube-controllers", "calico-typha"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, CalicoSystemNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
	By("Waiting for Ready calico-apiserver deployment pods")
	for _, d := range []string{"calico-apiserver"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, CalicoAPIServerNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
}

// CheckTestBeforeCleanup checks to see if the current running Ginkgo test failed, and prints
// a status message regarding cleanup.
func CheckTestBeforeCleanup() {
	if CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
		Logf("FAILED!")
	}
	Logf("Cleaning up after \"%s\" spec", CurrentSpecReport().FullText())
}
