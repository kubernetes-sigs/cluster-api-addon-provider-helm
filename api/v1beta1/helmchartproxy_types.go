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

const (
	// HelmChartProxyFinalizer is the finalizer used by the HelmChartProxy controller to cleanup add-on resources when
	// a HelmChartProxy is being deleted.
	HelmChartProxyFinalizer = "helmchartproxy.addons.cluster.x-k8s.io"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HelmChartProxySpec defines the desired state of HelmChartProxy.
type HelmChartProxySpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// ClusterSelector selects Clusters with a label that matches the specified key/value pair. The Helm chart will be
	// installed on all selected Clusters. If a Cluster is no longer selected, the Helm release will be uninstalled.
	ClusterSelector ClusterSelectorLabel `json:"clusterSelector"`

	// ChartName is the name of the Helm chart in the repository.
	ChartName string `json:"chartName,omitempty"`

	// RepoURL is the URL of the Helm chart repository.
	RepoURL string `json:"repoURL,omitempty"`

	// ReleaseName is the release name of the installed Helm chart. If it is not specified, a name will be generated.
	// +optional
	ReleaseName string `json:"releaseName,omitempty"`

	// Version is the version of the Helm chart. If it is not specified, the latest version will be used.
	// +optional
	Version string `json:"version,omitempty"`

	// Namespace is the namespace the Helm release will be installed on each selected Cluster. If it is not specified, the default namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	//  Values is a map of key/value pairs specifying values to be passed to the Helm chart. The map key is the full path
	// to the field, and the map value is the value to set. The map value supports Go templating to reference fields from
	// the selected workload Cluster.
	// +optional
	Values map[string]string `json:"values,omitempty"`
}

// ClusterSelectorLabel defines a key/value pair used to select Clusters with a label matching the specified key and value.
type ClusterSelectorLabel struct {
	// Key is the label key.
	Key string `json:"key"`

	// Value is the label value.
	Value string `json:"value"`
}

// HelmChartProxyStatus defines the observed state of HelmChartProxy.
type HelmChartProxyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready is true when the HelmReleaseProxySpec for each selected Cluster is up to date.
	// +optional
	Ready bool `json:"ready"`

	// MatchingClusters is the list of references to Clusters selected by the ClusterSelectorLabel.
	// +optional
	MatchingClusters []corev1.ObjectReference `json:"matchingClusters"`

	// FailureReason will be set in the event that there is a an error reconciling the HelmChartProxy.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready"
// +kubebuilder:subresource:status

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
