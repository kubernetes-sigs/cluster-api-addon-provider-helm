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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HelmChartProxySpec defines the desired state of HelmChartProxy
type HelmChartProxySpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// ClusterSelector selects Clusters with a label that matches the specified key/value pair. The Helm chart will be installed on
	// all selected Clusters. If a Cluster is no longer selected, the Helm release will be uninstalled.
	ClusterSelector ClusterSelectorLabel `json:"clusterSelector"`

	// ReleaseName is the release name of the installed Helm chart.
	ReleaseName string `json:"releaseName"`

	// Version is the version of the Helm chart. To be replaced with a compatibility matrix.
	Version string `json:"version,omitempty"`

	// ChartName is the name of the Helm chart in the repository.
	ChartName string `json:"chartName,omitempty"`

	// RepoURL is the URL of the Helm chart repository.
	RepoURL string `json:"repoURL,omitempty"`

	// Values is the set of key/value pair values that we pass to Helm. This field is currently used for testing and is subject to change.
	Values map[string]string `json:"values,omitempty"`
}

// ClusterSelectorLabel defines a key/value pair used to select Clusters with a label matching the specified key and value.
type ClusterSelectorLabel struct {
	// Key is the label key.
	Key string `json:"kind"`

	// Value is the label value.
	Value string `json:"value"`
}

// HelmChartProxyStatus defines the observed state of HelmChartProxy
type HelmChartProxyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// FailureReason will be set in the event that there is a an error reconciling the HelmChartProxy.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HelmChartProxy is the Schema for the helmchartproxies API
type HelmChartProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmChartProxySpec   `json:"spec,omitempty"`
	Status HelmChartProxyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HelmChartProxyList contains a list of HelmChartProxy
type HelmChartProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmChartProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HelmChartProxy{}, &HelmChartProxyList{})
}
