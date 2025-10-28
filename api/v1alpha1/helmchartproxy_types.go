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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// ReconcileStrategy is a string representation of the reconciliation strategy of a HelmChartProxy.
type ReconcileStrategy string

const (
	// HelmChartProxyFinalizer is the finalizer used by the HelmChartProxy controller to cleanup add-on resources when
	// a HelmChartProxy is being deleted.
	HelmChartProxyFinalizer = "helmchartproxy.addons.cluster.x-k8s.io"

	// DefaultOCIKey is the default file name of the OCI secret key.
	DefaultOCIKey = "config.json"

	// ReconcileStrategyContinuous is the default reconciliation strategy for HelmChartProxy. It will attempt to install the Helm
	// chart on a selected Cluster, update the Helm release to match the current HelmChartProxy spec, and delete the Helm release
	// if the Cluster no longer selected.
	ReconcileStrategyContinuous ReconcileStrategy = "Continuous"

	// ReconcileStrategyInstallOnce attempts to install the Helm chart for a HelmChartProxy on a selected Cluster, and once
	// it is installed, it will not attempt to update or delete the Helm release on the Cluster again.
	ReconcileStrategyInstallOnce ReconcileStrategy = "InstallOnce"
)

// HelmChartProxySpec defines the desired state of HelmChartProxy.
type HelmChartProxySpec struct {
	// ClusterSelector selects Clusters in the same namespace with a label that matches the specified label selector. The Helm
	// chart will be installed on all selected Clusters. If a Cluster is no longer selected, the Helm release will be uninstalled.
	ClusterSelector metav1.LabelSelector `json:"clusterSelector"`

	// ChartName is the name of the Helm chart in the repository.
	// e.g. chart-path oci://repo-url/chart-name as chartName: chart-name and https://repo-url/chart-name as chartName: chart-name
	ChartName string `json:"chartName"`

	// RepoURL is the URL of the Helm chart repository.
	// e.g. chart-path oci://repo-url/chart-name as repoURL: oci://repo-url and https://repo-url/chart-name as repoURL: https://repo-url
	RepoURL string `json:"repoURL"`

	// ReleaseName is the release name of the installed Helm chart. If it is not specified, a name will be generated.
	// +optional
	ReleaseName string `json:"releaseName,omitempty"`

	// ReleaseNamespace is the namespace the Helm release will be installed on each selected
	// Cluster. If it is not specified, it will be set to the default namespace.
	// +optional
	ReleaseNamespace string `json:"namespace,omitempty"`

	// Version is the version of the Helm chart. If it is not specified, the chart will use
	// and be kept up to date with the latest version.
	// +optional
	Version string `json:"version,omitempty"`

	// ValuesTemplate is an inline YAML representing the values for the Helm chart. This YAML supports Go templating to reference
	// fields from each selected workload Cluster and programatically create and set values.
	// +optional
	ValuesTemplate string `json:"valuesTemplate,omitempty"`

	// ReconcileStrategy indicates whether a Helm chart should be continuously installed, updated, and uninstalled on selected Clusters,
	// or if it should be reconciled until it is successfully installed on selected Clusters and not otherwise updated or uninstalled.
	// If not specified, the default behavior will be to reconcile continuously. This field is immutable.
	// Possible values are `Continuous`, `InstallOnce`, or unset.
	// +kubebuilder:validation:Enum="";InstallOnce;Continuous;
	// +optional
	ReconcileStrategy string `json:"reconcileStrategy,omitempty"`

	// Options represents CLI flags passed to Helm operations (i.e. install, upgrade, delete) and
	// include options such as wait, skipCRDs, timeout, waitForJobs, etc.
	// +optional
	Options HelmOptions `json:"options,omitempty"`

	// Credentials is a reference to an object containing the OCI credentials. If it is not specified, no credentials will be used.
	// +optional
	Credentials *Credentials `json:"credentials,omitempty"`

	// TLSConfig contains the TLS configuration for a HelmChartProxy.
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`
}

type HelmOptions struct {
	// DisableHooks prevents hooks from running during the Helm install action.
	// +optional
	DisableHooks bool `json:"disableHooks,omitempty"`

	// Wait enables the waiting for resources to be ready after a Helm install/upgrade has been performed.
	// +optional
	Wait bool `json:"wait,omitempty"`

	// WaitForJobs enables waiting for jobs to complete after a Helm install/upgrade has been performed.
	// +optional
	WaitForJobs bool `json:"waitForJobs,omitempty"`

	// DependencyUpdate indicates the Helm install/upgrade action to get missing dependencies.
	// +optional
	DependencyUpdate bool `json:"dependencyUpdate,omitempty"`

	// Timeout is the time to wait for any individual Kubernetes operation (like
	// resource creation, Jobs for hooks, etc.) during the performance of a Helm install action.
	// Defaults to '10 min'.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// SkipCRDs controls whether CRDs should be installed during install/upgrade operation.
	// By default, CRDs are installed if not already present.
	// If set, no CRDs will be installed.
	// +optional
	SkipCRDs bool `json:"skipCRDs,omitempty"`

	// SubNotes determines whether sub-notes should be rendered in the chart.
	// +optional
	SubNotes bool `json:"options,omitempty"`

	// DisableOpenAPIValidation controls whether OpenAPI validation is enforced.
	// +optional
	DisableOpenAPIValidation bool `json:"disableOpenAPIValidation,omitempty"`

	// Atomic indicates the installation/upgrade process to delete the installation or rollback on failure.
	// If 'Atomic' is set, wait will be enabled automatically during helm install/upgrade operation.
	// +optional
	Atomic bool `json:"atomic,omitempty"`

	// Install represents CLI flags passed to Helm install operation which can be used to control
	// behaviour of helm Install operations via options like wait, skipCrds, timeout, waitForJobs, etc.
	// +optional
	Install HelmInstallOptions `json:"install,omitempty"`

	// Upgrade represents CLI flags passed to Helm upgrade operation which can be used to control
	// behaviour of helm Upgrade operations via options like wait, skipCrds, timeout, waitForJobs, etc.
	// +optional
	Upgrade HelmUpgradeOptions `json:"upgrade,omitempty"`

	// Uninstall represents CLI flags passed to Helm uninstall operation which can be used to control
	// behaviour of helm Uninstall operation via options like wait, timeout, etc.
	// +optional
	Uninstall *HelmUninstallOptions `json:"uninstall,omitempty"`

	// EnableClientCache is a flag to enable Helm client cache. If it is not specified, it will be set to true.
	// +kubebuilder:default=false
	// +optional
	EnableClientCache bool `json:"enableClientCache,omitempty"`
}

type HelmInstallOptions struct {
	// CreateNamespace indicates the Helm install/upgrade action to create the
	// HelmChartProxySpec.ReleaseNamespace if it does not exist yet.
	// On uninstall, the namespace will not be garbage collected.
	// If it is not specified by user, will be set to default 'true'.
	// +kubebuilder:default=true
	// +optional
	CreateNamespace bool `json:"createNamespace,omitempty"`

	// IncludeCRDs determines whether CRDs stored as a part of helm templates directory should be installed.
	// +optional
	IncludeCRDs bool `json:"includeCRDs,omitempty"`
}

type HelmUpgradeOptions struct {
	// Force indicates to ignore certain warnings and perform the helm release upgrade anyway.
	// This should be used with caution.
	// +optional
	Force bool `json:"force,omitempty"`

	// ResetValues will reset the values to the chart's built-ins rather than merging with existing.
	// +optional
	ResetValues bool `json:"resetValues,omitempty"`

	// ReuseValues will re-use the user's last supplied values.
	// +optional
	ReuseValues bool `json:"reuseValues,omitempty"`

	// ResetThenReuseValues will reset the values to the chart's built-ins then merge with user's last supplied values.
	// +optional
	ResetThenReuseValues bool `json:"resetThenReuseValues,omitempty"`

	// Recreate will (if true) recreate pods after a rollback.
	// +optional
	Recreate bool `json:"recreate,omitempty"`

	// MaxHistory limits the maximum number of revisions saved per release (default is 10).
	// +kubebuilder:default=10
	// +optional
	MaxHistory int `json:"maxHistory,omitempty"`

	// CleanupOnFail indicates the upgrade action to delete newly-created resources on a failed update operation.
	// +optional
	CleanupOnFail bool `json:"cleanupOnFail,omitempty"`
}

type HelmUninstallOptions struct {
	// KeepHistory defines whether historical revisions of a release should be saved.
	// If it's set, helm uninstall operation will not delete the history of the release.
	// The helm storage backend (secret, configmap, etc) will be retained in the cluster.
	// +optional
	KeepHistory bool `json:"keepHistory,omitempty"`

	// Description represents human readable information to be shown on release uninstall.
	// +optional
	Description string `json:"description,omitempty"`
}

type Credentials struct {
	// Secret is a reference to a Secret containing the OCI credentials.
	Secret corev1.SecretReference `json:"secret"`

	// Key is the key in the Secret containing the OCI credentials.
	Key string `json:"key"`
}

// TLSConfig defines a TLS configuration.
type TLSConfig struct {
	// Secret is a reference to a Secret containing the TLS CA certificate at the key ca.crt.
	// +optional
	CASecretRef *corev1.SecretReference `json:"caSecret,omitempty"`

	// InsecureSkipTLSVerify controls whether the Helm client should verify the server's certificate.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`
}

// HelmChartProxyStatus defines the observed state of HelmChartProxy.
type HelmChartProxyStatus struct {
	// Conditions defines current state of the HelmChartProxy.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// MatchingClusters is the list of references to Clusters selected by the ClusterSelector.
	// +optional
	MatchingClusters []corev1.ObjectReference `json:"matchingClusters"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:resource:shortName=hcp

// HelmChartProxy is the Schema for the helmchartproxies API.
type HelmChartProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmChartProxySpec   `json:"spec,omitempty"`
	Status HelmChartProxyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HelmChartProxyList contains a list of HelmChartProxy.
type HelmChartProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmChartProxy `json:"items"`
}

// GetConditions returns the list of conditions for an HelmChartProxy API object.
func (c *HelmChartProxy) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an HelmChartProxy object.
func (c *HelmChartProxy) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// SetMatchingClusters will set the given list of matching clusters on an HelmChartProxy object.
func (c *HelmChartProxy) SetMatchingClusters(clusterList []clusterv1.Cluster) {
	matchingClusters := make([]corev1.ObjectReference, 0, len(clusterList))
	for _, cluster := range clusterList {
		matchingClusters = append(matchingClusters, corev1.ObjectReference{
			Kind:       cluster.Kind,
			APIVersion: cluster.APIVersion,
			Name:       cluster.Name,
			Namespace:  cluster.Namespace,
		})
	}

	c.Status.MatchingClusters = matchingClusters
}

func init() {
	SchemeBuilder.Register(&HelmChartProxy{}, &HelmChartProxyList{})
}
