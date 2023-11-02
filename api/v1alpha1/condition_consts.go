/*
Copyright 2022 The Kubernetes Authors.

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

package v1alpha1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// HelmChartProxy Conditions and Reasons.
const (
	// HelmReleaseProxySpecsUpToDateCondition indicates that the HelmReleaseProxy specs are up to date with the HelmChartProxy specs,
	// meaning that the HelmReleaseProxies are created/updated, value template parsing succeeded, and the orphaned HelmReleaseProxies are deleted.
	HelmReleaseProxySpecsUpToDateCondition clusterv1.ConditionType = "HelmReleaseProxySpecsUpToDate"

	// HelmReleaseProxyCreationFailedReason indicates that the HelmChartProxy controller failed to create a HelmReleaseProxy.
	HelmReleaseProxyCreationFailedReason = "HelmReleaseProxyCreationFailed"

	// HelmReleaseProxyDeletionFailedReason indicates that the HelmChartProxy controller failed to delete a HelmReleaseProxy.
	HelmReleaseProxyDeletionFailedReason = "HelmReleaseProxyDeletionFailed"

	// HelmReleaseProxyReinstallingReason indicates that the HelmChartProxy controller is reinstalling a HelmReleaseProxy.
	HelmReleaseProxyReinstallingReason = "HelmReleaseProxyReinstalling"

	// ValueParsingFailedReason indicates that the HelmChartProxy controller failed to parse the values.
	ValueParsingFailedReason = "ValueParsingFailed"

	// ClusterSelectionFailedReason indicates that the HelmChartProxy controller failed to select the workload Clusters.
	ClusterSelectionFailedReason = "ClusterSelectionFailed"

	// HelmReleaseProxiesReadyCondition indicates that the HelmReleaseProxies are ready, meaning that the Helm installation, upgrade
	// or deletion is complete.
	HelmReleaseProxiesReadyCondition clusterv1.ConditionType = "HelmReleaseProxiesReady"
)

// HelmReleaseProxy Conditions and Reasons.
const (
	// HelmReleaseReadyCondition indicates the current status of the underlying Helm release managed by the HelmReleaseProxy.
	HelmReleaseReadyCondition clusterv1.ConditionType = "HelmReleaseReady"

	// PreparingToHelmInstallReason indicates that the HelmReleaseProxy is preparing to install the Helm release.
	PreparingToHelmInstallReason = "PreparingToHelmInstall"

	// HelmReleasePendingReason indicates that the HelmReleaseProxy is pending either install, upgrade, or rollback.
	HelmReleasePendingReason = "HelmReleasePending"

	// HelmInstallOrUpgradeFailedReason indicates that the HelmReleaseProxy failed to install or upgrade the Helm release.
	HelmInstallOrUpgradeFailedReason = "HelmInstallOrUpgradeFailed"

	// HelmReleaseDeletionFailedReason is indicates that the HelmReleaseProxy failed to delete the Helm release.
	HelmReleaseDeletionFailedReason = "HelmReleaseDeletionFailed"

	// HelmReleaseDeletedReason indicates that the HelmReleaseProxy deleted the Helm release.
	HelmReleaseDeletedReason = "HelmReleaseDeleted"

	// HelmReleaseGetFailedReason indicates that the HelmReleaseProxy failed to get the Helm release.
	HelmReleaseGetFailedReason = "HelmReleaseGetFailed"

	// ClusterAvailableCondition indicates that the Cluster to install the Helm release on is available.
	ClusterAvailableCondition clusterv1.ConditionType = "ClusterAvailable"

	// GetClusterFailedReason indicates that the HelmReleaseProxy failed to get the Cluster.
	GetClusterFailedReason = "GetClusterFailed"

	// GetKubeconfigFailedReason indicates that the HelmReleaseProxy failed to get the kubeconfig for the Cluster.
	GetKubeconfigFailedReason = "GetKubeconfigFailed"

	// GetCredentialsFailedReason indicates that the HelmReleaseProxy failed to get the credentials for the Helm registry.
	GetCredentialsFailedReason = "GetCredentialsFailed"
)
