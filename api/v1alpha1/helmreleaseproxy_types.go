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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// HelmReleaseProxyFinalizer is the finalizer used by the HelmReleaseProxy controller to cleanup add-on resources when
	// a HelmReleaseProxy is being deleted.
	HelmReleaseProxyFinalizer = "helmreleaseproxy.addons.cluster.x-k8s.io"

	// HelmChartProxyLabelName is the label signifying which HelmChartProxy a HelmReleaseProxy is associated with.
	HelmChartProxyLabelName = "helmreleaseproxy.addons.cluster.x-k8s.io/helmchartproxy-name"

	// IsReleaseNameGeneratedAnnotation is the annotation signifying the Helm release name is auto-generated.
	IsReleaseNameGeneratedAnnotation = "helmreleaseproxy.addons.cluster.x-k8s.io/is-release-name-generated"
)

// HelmReleaseProxySpec defines the desired state of HelmReleaseProxy.
type HelmReleaseProxySpec struct {
	// ClusterRef is a reference to the Cluster to install the Helm release on.
	ClusterRef corev1.ObjectReference `json:"clusterRef"`

	// ChartName is the name of the Helm chart in the repository.
	// e.g. chart-path oci://repo-url/chart-name as chartName: chart-name and https://repo-url/chart-name as chartName: chart-name
	ChartName string `json:"chartName"`

	// RepoURL is the URL of the Helm chart repository.
	// e.g. chart-path oci://repo-url/chart-name as repoURL: oci://repo-url and https://repo-url/chart-name as repoURL: https://repo-url
	RepoURL string `json:"repoURL"`

	// ReleaseName is the release name of the installed Helm chart. If it is not specified, a name will be generated.
	// +optional
	ReleaseName string `json:"releaseName,omitempty"`

	// ReleaseNamespace is the namespace the Helm release will be installed on the referenced
	// Cluster. If it is not specified, it will be set to the default namespace.
	// +optional
	ReleaseNamespace string `json:"namespace"`

	// Version is the version of the Helm chart. If it is not specified, the chart will use
	// and be kept up to date with the latest version.
	// +optional
	Version string `json:"version,omitempty"`

	// Values is an inline YAML representing the values for the Helm chart. This YAML is the result of the rendered
	// Go templating with the values from the referenced workload Cluster.
	// +optional
	Values string `json:"values,omitempty"`

	// Options represents the helm setting options which can be used to control behaviour of helm operations(Install, Upgrade, Delete, etc)
	// via options like wait, skipCrds, timeout, waitForJobs, etc.
	// +optional
	Options *HelmOptions `json:"options,omitempty"`
}

// HelmReleaseProxyStatus defines the observed state of HelmReleaseProxy.
type HelmReleaseProxyStatus struct {
	// Conditions defines current state of the HelmReleaseProxy.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// Status is the current status of the Helm release.
	// +optional
	Status string `json:"status,omitempty"`

	// Revision is the current revision of the Helm release.
	// +optional
	Revision int `json:"revision,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterRef.name",description="Cluster to which this HelmReleaseProxy belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=".status.revision"
// +kubebuilder:resource:shortName=hrp

// HelmReleaseProxy is the Schema for the helmreleaseproxies API.
type HelmReleaseProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmReleaseProxySpec   `json:"spec,omitempty"`
	Status HelmReleaseProxyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HelmReleaseProxyList contains a list of HelmReleaseProxy.
type HelmReleaseProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmReleaseProxy `json:"items"`
}

// GetConditions returns the list of conditions for an HelmReleaseProxy API object.
func (r *HelmReleaseProxy) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions will set the given conditions on an HelmReleaseProxy object.
func (r *HelmReleaseProxy) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

// SetReleaseStatus will set the given status on an HelmReleaseProxy object.
func (r *HelmReleaseProxy) SetReleaseStatus(status string) {
	r.Status.Status = status // See pkg/release/status.go in Helm for possible values
	// r.Status.Status = release.Info.Status.String() // See pkg/release/status.go in Helm for possible values
}

// SetReleaseRevision will set the given revision on an HelmReleaseProxy object.
func (r *HelmReleaseProxy) SetReleaseRevision(version int) {
	r.Status.Revision = version
}

// SetReleaseName will set the given name on an HelmReleaseProxy object. This is used if the release name is auto-generated by Helm.
func (r *HelmReleaseProxy) SetReleaseName(name string) {
	if r.Spec.ReleaseName == "" {
		r.Spec.ReleaseName = name
	}
}

func init() {
	SchemeBuilder.Register(&HelmReleaseProxy{}, &HelmReleaseProxyList{})
}
