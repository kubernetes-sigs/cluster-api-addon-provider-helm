/*
Copyright 2022.

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

package v1beta1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// HelmChartProxy Conditions and Reasons.
const (
	// ReconciliationErrorReason is used to indicate there was some error in reconciliation.
	ReconciliationErrorReason = "ReconciliationError"

	// HelmReleaseProxySpecsReadyCondition...
	HelmReleaseProxySpecsReadyCondition clusterv1.ConditionType = "HelmReleaseProxySpecsReady"

	// // HelmReleaseProxiesReadyCondition...
	// HelmReleaseProxiesReadyCondition clusterv1.ConditionType = "HelmReleaseProxiesReady"

	// HelmReleaseProxyCreationFailedReason...
	HelmReleaseProxyCreationFailedReason = "HelmReleaseProxyCreationFailed"
	// HelmReleaseProxyDeletionFailedReason...
	HelmReleaseProxyDeletionFailedReason = "HelmReleaseProxyDeletionFailed"

	// ClusterSelectionSucceededCondition...
	ClusterSelectionSucceededCondition clusterv1.ConditionType = "ClusterSelectionSucceeded"
	// ClusterSelectionFailedReason is ...
	ClusterSelectionFailedReason = "ClusterSelectionFailed"

	// ValueParsingSucceededCondition...
	ValueParsingSucceededCondition clusterv1.ConditionType = "ValueParsingSucceeded"

	// ValueParsingFailedReason is ...
	ValueParsingFailedReason = "ValueParsingFailed"
)

// HelmReleaseProxy Conditions and Reasons.
const (
	// HelmReleaseReadyCondition reports on current status of the HelmRelease managed by the HelmChartProxy.
	HelmReleaseReadyCondition clusterv1.ConditionType = "HelmReleaseReady"
	// HelmInstallOrUpgradeFailedReason is ...
	HelmInstallOrUpgradeFailedReason = "HelmInstallOrUpgradeFailed"
	// HelmReleaseDeletionFailedReason is ...
	HelmReleaseDeletionFailedReason = "HelmReleaseDeletionFailed"
	// HelmReleaseDeletedReason is ...
	HelmReleaseDeletedReason = "HelmReleaseDeleted"
	// HelmReleaseGetFailedReason is ...
	HelmReleaseGetFailedReason = "HelmReleaseGetFailed"

	// ClusterAvailableCondition...
	ClusterAvailableCondition clusterv1.ConditionType = "ClusterAvailable"
	// GetClusterFailedReason is ...
	GetClusterFailedReason = "GetClusterFailed"
	// GetKubeconfigFailedReason is ...
	GetKubeconfigFailedReason = "GetKubeconfigFailed"
)
