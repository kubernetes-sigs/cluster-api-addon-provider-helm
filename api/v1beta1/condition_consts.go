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
	// HelmReleaseProxySpecsUpToDateCondition...
	HelmReleaseProxySpecsUpToDateCondition clusterv1.ConditionType = "HelmReleaseProxySpecsUpToDate"
	// GenericErrorReason is used to indicate there was some error in reconciliation.
	GenericErrorReason = "GenericError"

	// ClusterSelectionSucceededCondition...
	ClusterSelectionSucceededCondition clusterv1.ConditionType = "ClusterSelectionSucceeded"

	// ReinstallInProgressReason...
	ReinstallInProgressReason = "ReinstallInProgress"

	// ClusterSelectionFailedReason is ...
	ClusterSelectionFailedReason = "ClusterSelectionFailed"
	// ValueParsingFailedReason is ...
	ValueParsingFailedReason = "ValueParsingFailed"
)

// HelmReleaseProxy Conditions and Reasons.
const (
	// HelmReleaseUpToDateCondition reports on current status of the HelmRelease managed by the HelmChartProxy.
	HelmReleaseUpToDateCondition clusterv1.ConditionType = "HelmReleaseUpToDate"
	// HelmInstallFailedReason is ...
	HelmInstallFailedReason = "HelmInstallFailed"
	// HelmUpgradeFailedReason is ...
	HelmUpgradeFailedReason = "HelmUpgradeFailed"
	// HelmReleaseDeletedReason is ...
	HelmReleaseDeletedReason = "HelmReleaseDeleted"

	// ClusterAvailableCondition...
	ClusterAvailableCondition clusterv1.ConditionType = "ClusterAvailable"
	// GetClusterFailedReason is ...
	GetClusterFailedReason = "GetClusterFailed"
	// GetKubeconfigFailedReason is ...
	GetKubeconfigFailedReason = "GetKubeconfigFailed"
)
