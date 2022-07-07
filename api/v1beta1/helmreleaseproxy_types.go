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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HelmReleaseProxySpec defines the desired state of HelmReleaseProxy
type HelmReleaseProxySpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// ClusterRef is the name of the cluster to deploy to
	ClusterRef *corev1.ObjectReference `json:"clusterRef,omitempty"`

	// ReleaseName is the release name of the installed Helm chart.
	ReleaseName string `json:"releaseName"`

	// Version is the version of the Helm chart. To be replaced with a compatibility matrix.
	// +optional
	Version string `json:"version,omitempty"`

	// ChartName is the name of the Helm chart in the repository.
	ChartName string `json:"chartName,omitempty"`

	// RepoURL is the URL of the Helm chart repository.
	RepoURL string `json:"repoURL,omitempty"`

	// Values is the set of key/value pair values that we pass to Helm. This field is currently used for testing and is subject to change.
	// +optional
	Values map[string]string `json:"values,omitempty"`
}

// HelmReleaseProxyStatus defines the observed state of HelmReleaseProxy
type HelmReleaseProxyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// FailureReason will be set in the event that there is a an error reconciling the HelmReleaseProxy.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterRef.name",description="Cluster to which this HelmReleaseProxy belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready"
// +kubebuilder:subresource:status

// HelmReleaseProxy is the Schema for the helmreleaseproxies API
type HelmReleaseProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmReleaseProxySpec   `json:"spec,omitempty"`
	Status HelmReleaseProxyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HelmReleaseProxyList contains a list of HelmReleaseProxy
type HelmReleaseProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmReleaseProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HelmReleaseProxy{}, &HelmReleaseProxyList{})
}
