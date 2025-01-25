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
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"sigs.k8s.io/cluster-api-addon-provider-helm/controllers/helmreleasedrift"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api/test/framework"
)

type ValidationType string

const (
	ValidationEventually   ValidationType = "Eventually"
	ValidationConsistently ValidationType = "Consistently"
)

// HelmReleaseDriftInput specifies the input for Helm release drift Deployment validation and verifying that it was successful.
type HelmReleaseDriftInput struct {
	BootstrapClusterProxy      framework.ClusterProxy
	Namespace                  *corev1.Namespace
	ClusterName                string
	HelmChartProxy             *addonsv1alpha1.HelmChartProxy
	UpdatedDeploymentReplicas  int32
	ExpectedDeploymentReplicas int32
	ExpectedRevision           int
	Validation                 ValidationType
}

func HelmReleaseDriftWithDeployment(ctx context.Context, inputGetter func() HelmReleaseDriftInput) {
	var (
		specName   = "helm-upgrade"
		input      HelmReleaseDriftInput
		mgmtClient ctrlclient.Client
	)
	input = inputGetter()
	hcp := input.HelmChartProxy
	Expect(input.Validation).NotTo(BeEmpty(), "HelmReleaseDriftInput must contains validation type to be defined")

	// Get workload Cluster proxy
	By("creating a clusterctl proxy to the workload cluster")
	workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())
	mgmtClient = workloadClusterProxy.GetClient()

	deploymentList := &appsv1.DeploymentList{}
	err := mgmtClient.List(ctx, deploymentList, ctrlclient.InNamespace(hcp.Spec.ReleaseNamespace), ctrlclient.MatchingLabels{helmreleasedrift.InstanceLabelKey: hcp.Spec.ReleaseName})
	Expect(err).NotTo(HaveOccurred())
	Expect(deploymentList.Items).NotTo(BeEmpty())

	deployment := &deploymentList.Items[0]
	patch := ctrlclient.MergeFrom(deployment.DeepCopy())
	deployment.Spec.Replicas = ptr.To(input.UpdatedDeploymentReplicas)
	err = mgmtClient.Patch(ctx, deployment, patch)
	Expect(err).NotTo(HaveOccurred())

	deploymentName := ctrlclient.ObjectKeyFromObject(deployment)
	if input.Validation == ValidationEventually {
		// Wait for Helm release Deployment replicas to be returned back
		Eventually(func() error {
			if err = mgmtClient.Get(ctx, deploymentName, deployment); err != nil {
				return err
			}
			if *deployment.Spec.Replicas != input.ExpectedDeploymentReplicas && deployment.Status.ReadyReplicas != input.ExpectedDeploymentReplicas {
				return fmt.Errorf("expected Deployment replicas to be %d, got %d", input.ExpectedDeploymentReplicas, deployment.Status.ReadyReplicas)
			}

			return nil
		}, e2eConfig.GetIntervals("default", "wait-helm-release-drift")...).Should(Succeed())
	}
	if input.Validation == ValidationConsistently {
		Consistently(func() error {
			if err = mgmtClient.Get(ctx, deploymentName, deployment); err != nil {
				return err
			}
			if *deployment.Spec.Replicas != input.ExpectedDeploymentReplicas && deployment.Status.ReadyReplicas != input.ExpectedDeploymentReplicas {
				return fmt.Errorf("expected Deployment replicas to be %d, got %d", input.ExpectedDeploymentReplicas, deployment.Status.ReadyReplicas)
			}

			return nil
		}, e2eConfig.GetIntervals("default", "wait-helm-release-drift")...).Should(Succeed())
	}

	helmUpgradeInput := HelmUpgradeInput{
		BootstrapClusterProxy: workloadClusterProxy,
		Namespace:             input.Namespace,
		ClusterName:           input.ClusterName,
		HelmChartProxy:        input.HelmChartProxy,
		ExpectedRevision:      input.ExpectedRevision,
	}
	EnsureHelmReleaseInstallOrUpgrade(ctx, specName, input.BootstrapClusterProxy, nil, &helmUpgradeInput, true)
}
