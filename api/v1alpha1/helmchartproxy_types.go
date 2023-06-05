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
	// HelmChartProxyFinalizer is the finalizer used by the HelmChartProxy controller to cleanup add-on resources when
	// a HelmChartProxy is being deleted.
	HelmChartProxyFinalizer = "helmchartproxy.addons.cluster.x-k8s.io"
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
}

// HelmChartProxyStatus defines the observed state of HelmChartProxy.
type HelmChartProxyStatus struct {
	// Conditions defines current state of the HelmChartProxy.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// MatchingClusters is the list of references to Clusters selected by the ClusterSelector.
	// +optional
	MatchingClusters []corev1.ObjectReference `json:"matchingClusters"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:resource:shortName=hcp

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

// GetConditions returns the list of conditions for an HelmChartProxy API object.
func (c *HelmChartProxy) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an HelmChartProxy object.
func (c *HelmChartProxy) SetConditions(conditions clusterv1.Conditions) {
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
