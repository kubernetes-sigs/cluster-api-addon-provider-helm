/*
Copyright 2023 The Kubernetes Authors.

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

package helmchartproxy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = &HelmChartProxyReconciler{}

var (
	ctx        = ctrl.SetupSignalHandler()
	fakeScheme = runtime.NewScheme()

	continuousProxy = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "test-value",
				},
			},
			ReleaseName:       "test-release-name",
			ChartName:         "test-chart-name",
			RepoURL:           "https://test-repo-url",
			ReleaseNamespace:  "test-release-namespace",
			Version:           "test-version",
			ValuesTemplate:    "apiServerPort: {{ .Cluster.spec.clusterNetwork.apiServerPort }}",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyContinuous),
			Options:           addonsv1alpha1.HelmOptions{},
		},
	}

	rolloutStepSizeProxy = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "test-value",
				},
			},
			RolloutStepSize:   &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
			ReleaseName:       "test-release-name",
			ChartName:         "test-chart-name",
			RepoURL:           "https://test-repo-url",
			ReleaseNamespace:  "test-release-namespace",
			Version:           "test-version",
			ValuesTemplate:    "apiServerPort: {{ .Cluster.spec.clusterNetwork.apiServerPort }}",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyContinuous),
			Options:           addonsv1alpha1.HelmOptions{},
		},
	}

	rolloutStepSizeProxyWithReleaseProxyReadyFalse = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "test-value",
				},
			},
			RolloutStepSize:   &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
			ReleaseName:       "test-release-name",
			ChartName:         "test-chart-name",
			RepoURL:           "https://test-repo-url",
			ReleaseNamespace:  "test-release-namespace",
			Version:           "test-version",
			ValuesTemplate:    "apiServerPort: {{ .Cluster.spec.clusterNetwork.apiServerPort }}",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyContinuous),
			Options:           addonsv1alpha1.HelmOptions{},
		},
		Status: addonsv1alpha1.HelmChartProxyStatus{
			Conditions: []clusterv1.Condition{
				{
					Type:     addonsv1alpha1.HelmReleaseProxiesReadyCondition,
					Status:   corev1.ConditionFalse,
					Severity: clusterv1.ConditionSeverityInfo,
				},
			},
		},
	}

	unsetProxy = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "test-value",
				},
			},
			ReleaseName:      "test-release-name",
			ChartName:        "test-chart-name",
			RepoURL:          "https://test-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			ValuesTemplate:   "apiServerPort: {{ .Cluster.spec.clusterNetwork.apiServerPort }}",
			Options:          addonsv1alpha1.HelmOptions{},
		},
	}

	installOnceProxy = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "test-value",
				},
			},
			ReleaseName:       "test-release-name",
			ChartName:         "test-chart-name",
			RepoURL:           "https://test-repo-url",
			ReleaseNamespace:  "test-release-namespace",
			Version:           "test-version",
			ValuesTemplate:    "apiServerPort: {{ .Cluster.spec.clusterNetwork.apiServerPort }}",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyInstallOnce),
			Options:           addonsv1alpha1.HelmOptions{},
		},
	}

	updatedInstallOnceProxy = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "test-value",
				},
			},
			ReleaseName:       "test-release-name",
			ChartName:         "test-chart-name",
			RepoURL:           "https://test-repo-url",
			ReleaseNamespace:  "test-release-namespace",
			Version:           "test-version",
			ValuesTemplate:    "serviceDomain: {{ .Cluster.spec.clusterNetwork.serviceDomain }}",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyInstallOnce),
			Options:           addonsv1alpha1.HelmOptions{},
		},
	}

	cluster1 = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-1",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"test-label": "test-value",
			},
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				APIServerPort: ptr.To(int32(1234)),
				ServiceDomain: "test-domain-1",
			},
		},
	}

	cluster2 = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-2",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"test-label":  "test-value",
				"other-label": "other-value",
			},
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				APIServerPort: ptr.To(int32(5678)),
				ServiceDomain: "test-domain-2",
			},
		},
	}

	cluster3 = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-3",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"other-label": "other-value",
			},
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				APIServerPort: ptr.To(int32(6443)),
				ServiceDomain: "test-domain-3",
			},
		},
	}

	cluster4 = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-4",
			Namespace: "other-namespace",
			Labels: map[string]string{
				"other-label": "other-value",
			},
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				APIServerPort: ptr.To(int32(6443)),
				ServiceDomain: "test-domain-4",
			},
		},
	}

	clusterPaused = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-paused",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"test-label": "test-value",
			},
		},
		Spec: clusterv1.ClusterSpec{
			Paused: true,
		},
	}

	hrpReady1 = &addonsv1alpha1.HelmReleaseProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hrp-1",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         addonsv1alpha1.GroupVersion.String(),
					Kind:               "HelmChartProxy",
					Name:               "test-hcp",
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:             "test-cluster-1",
				addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
			},
			Annotations: map[string]string{
				addonsv1alpha1.ReleaseSuccessfullyInstalledAnnotation: "true",
			},
		},
		Spec: addonsv1alpha1.HelmReleaseProxySpec{
			ClusterRef: corev1.ObjectReference{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
				Name:       "test-cluster-1",
				Namespace:  "test-namespace",
			},
			ReleaseName:      "test-release-name",
			ChartName:        "test-chart-name",
			RepoURL:          "https://test-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			Values:           "apiServerPort: 1234",
			Options:          addonsv1alpha1.HelmOptions{},
		},
		Status: addonsv1alpha1.HelmReleaseProxyStatus{
			Conditions: []clusterv1.Condition{
				{
					Type:   clusterv1.ReadyCondition,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	hrpNotReady1 = &addonsv1alpha1.HelmReleaseProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hrp-1",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         addonsv1alpha1.GroupVersion.String(),
					Kind:               "HelmChartProxy",
					Name:               "test-hcp",
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:             "test-cluster-1",
				addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
			},
			Annotations: map[string]string{
				addonsv1alpha1.ReleaseSuccessfullyInstalledAnnotation: "true",
			},
		},
		Spec: addonsv1alpha1.HelmReleaseProxySpec{
			ClusterRef: corev1.ObjectReference{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
				Name:       "test-cluster-1",
				Namespace:  "test-namespace",
			},
			ReleaseName:      "test-release-name",
			ChartName:        "test-chart-name",
			RepoURL:          "https://test-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			Values:           "apiServerPort: 1234",
			Options:          addonsv1alpha1.HelmOptions{},
		},
		Status: addonsv1alpha1.HelmReleaseProxyStatus{
			Conditions: []clusterv1.Condition{
				{
					Type:     clusterv1.ReadyCondition,
					Status:   corev1.ConditionFalse,
					Severity: clusterv1.ConditionSeverityInfo,
				},
			},
		},
	}

	hrpReady2 = &addonsv1alpha1.HelmReleaseProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hrp-2",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         addonsv1alpha1.GroupVersion.String(),
					Kind:               "HelmChartProxy",
					Name:               "test-hcp",
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:             "test-cluster-2",
				addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
			},
			Annotations: map[string]string{
				addonsv1alpha1.ReleaseSuccessfullyInstalledAnnotation: "true",
			},
		},
		Spec: addonsv1alpha1.HelmReleaseProxySpec{
			ClusterRef: corev1.ObjectReference{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
				Name:       "test-cluster-2",
				Namespace:  "test-namespace",
			},
			ReleaseName:      "test-release-name",
			ChartName:        "test-chart-name",
			RepoURL:          "https://test-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			Values:           "apiServerPort: 5678",
			Options:          addonsv1alpha1.HelmOptions{},
		},
		Status: addonsv1alpha1.HelmReleaseProxyStatus{
			Conditions: []clusterv1.Condition{
				{
					Type:   clusterv1.ReadyCondition,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	hrpNotReady2 = &addonsv1alpha1.HelmReleaseProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hrp-2",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         addonsv1alpha1.GroupVersion.String(),
					Kind:               "HelmChartProxy",
					Name:               "test-hcp",
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:             "test-cluster-2",
				addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
			},
			Annotations: map[string]string{
				addonsv1alpha1.ReleaseSuccessfullyInstalledAnnotation: "true",
			},
		},
		Spec: addonsv1alpha1.HelmReleaseProxySpec{
			ClusterRef: corev1.ObjectReference{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
				Name:       "test-cluster-2",
				Namespace:  "test-namespace",
			},
			ReleaseName:      "test-release-name",
			ChartName:        "test-chart-name",
			RepoURL:          "https://test-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			Values:           "apiServerPort: 5678",
			Options:          addonsv1alpha1.HelmOptions{},
		},
		Status: addonsv1alpha1.HelmReleaseProxyStatus{
			Conditions: []clusterv1.Condition{
				{
					Type:     clusterv1.ReadyCondition,
					Status:   corev1.ConditionFalse,
					Severity: clusterv1.ConditionSeverityInfo,
				},
			},
		},
	}
)

func TestReconcileNormal(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name           string
		helmChartProxy *addonsv1alpha1.HelmChartProxy
		objects        []client.Object
		expect         func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy)
		expectedError  string
	}{
		{
			name:           "successfully select clusters and install HelmReleaseProxies for Continuous strategy",
			helmChartProxy: continuousProxy,
			objects:        []client.Object{cluster1, cluster2, cluster3, cluster4},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				// This is false as the HelmReleaseProxies won't be ready until the HelmReleaseProxy controller runs.
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeFalse())
			},
			expectedError: "",
		},
		{
			name:           "successfully select clusters and set Rollout Step Ready Condition as False",
			helmChartProxy: rolloutStepSizeProxy,
			objects:        []client.Object{cluster1, cluster2, cluster3, cluster4},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeFalse())
				// This is false as the HelmReleaseProxies won't be ready until the HelmReleaseProxy controller runs.
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeFalse())
			},
			expectedError: "",
		},
		{
			name:           "successfully select clusters and install HelmReleaseProxies for unset strategy",
			helmChartProxy: unsetProxy,
			objects:        []client.Object{cluster1, cluster2, cluster3, cluster4},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				// This is false as the HelmReleaseProxies won't be ready until the HelmReleaseProxy controller runs.
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeFalse())
			},
			expectedError: "",
		},
		{
			name:           "successfully select clusters and install HelmReleaseProxies for InstallOnce strategy",
			helmChartProxy: installOnceProxy,
			objects:        []client.Object{cluster1, cluster2, cluster3, cluster4},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				// This is false as the HelmReleaseProxies won't be ready until the HelmReleaseProxy controller runs.
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeFalse())
			},
			expectedError: "",
		},
		{
			name:           "mark HelmChartProxy as ready once HelmReleaseProxies ready conditions are true for Continuous strategy",
			helmChartProxy: continuousProxy,
			objects:        []client.Object{cluster1, cluster2, hrpReady1, hrpReady2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			name:           "mark HelmChartProxy as ready once HelmReleaseProxies ready conditions are true for unset strategy",
			helmChartProxy: unsetProxy,
			objects:        []client.Object{cluster1, cluster2, hrpReady1, hrpReady2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			// TODO: how to make sure HelmReleaseProxySpecsUpToDateCondition stays true even after they get deleted?
			name:           "mark HelmChartProxy as ready once HelmReleaseProxies ready conditions are true for InstallOnce strategy",
			helmChartProxy: installOnceProxy,
			objects:        []client.Object{cluster1, cluster2, hrpReady1, hrpReady2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			name:           "mark HelmChartProxy as ready once HelmReleaseProxies are rolled out and ready",
			helmChartProxy: rolloutStepSizeProxy,
			objects:        []client.Object{cluster1, cluster2, hrpReady1, hrpReady2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			name:           "mark HelmChartProxy as not-ready when not all helm release proxies are rolled out",
			helmChartProxy: rolloutStepSizeProxyWithReleaseProxyReadyFalse,
			objects:        []client.Object{cluster1, cluster2, hrpNotReady1},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeFalse())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeFalse())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeFalse())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			name:           "mark HelmChartProxy as not-ready when all helm release proxies are rolled out and not ready",
			helmChartProxy: rolloutStepSizeProxyWithReleaseProxyReadyFalse,
			objects:        []client.Object{cluster1, cluster2, hrpNotReady1, hrpNotReady2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeFalse())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesRolloutCompletedCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeFalse())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			name:           "successfully delete orphaned HelmReleaseProxies for Continuous strategy",
			helmChartProxy: continuousProxy,
			objects:        []client.Object{hrpReady1, hrpReady2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEmpty())
				g.Expect(c.Get(ctx, client.ObjectKey{Namespace: hrpReady1.Namespace, Name: hrpReady1.Name}, &addonsv1alpha1.HelmReleaseProxy{})).ToNot(Succeed())
				g.Expect(c.Get(ctx, client.ObjectKey{Namespace: hrpReady2.Namespace, Name: hrpReady2.Name}, &addonsv1alpha1.HelmReleaseProxy{})).ToNot(Succeed())

				// Vacuously true as there are no HRPs
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			name:           "successfully delete orphaned HelmReleaseProxies for unset strategy",
			helmChartProxy: unsetProxy,
			objects:        []client.Object{hrpReady1, hrpReady2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEmpty())
				g.Expect(c.Get(ctx, client.ObjectKey{Namespace: hrpReady1.Namespace, Name: hrpReady1.Name}, &addonsv1alpha1.HelmReleaseProxy{})).ToNot(Succeed())
				g.Expect(c.Get(ctx, client.ObjectKey{Namespace: hrpReady2.Namespace, Name: hrpReady2.Name}, &addonsv1alpha1.HelmReleaseProxy{})).ToNot(Succeed())

				// Vacuously true as there are no HRPs
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			name:           "do not delete orphaned HelmReleaseProxies for InstallOnce strategy",
			helmChartProxy: installOnceProxy,
			objects:        []client.Object{hrpReady1, hrpReady2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEmpty())
				g.Expect(c.Get(ctx, client.ObjectKey{Namespace: hrpReady1.Namespace, Name: hrpReady1.Name}, &addonsv1alpha1.HelmReleaseProxy{})).To(Succeed())
				g.Expect(c.Get(ctx, client.ObjectKey{Namespace: hrpReady2.Namespace, Name: hrpReady2.Name}, &addonsv1alpha1.HelmReleaseProxy{})).To(Succeed())

				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			name:           "do not update HelmReleaseProxies when HelmChartProxy changes for InstallOnce strategy",
			helmChartProxy: updatedInstallOnceProxy,
			objects:        []client.Object{hrpReady1, hrpReady2, cluster1, cluster2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
				}))

				hrp1Result := &addonsv1alpha1.HelmReleaseProxy{}
				g.Expect(c.Get(ctx, client.ObjectKey{Namespace: hrpReady1.Namespace, Name: hrpReady1.Name}, hrp1Result)).To(Succeed())
				g.Expect(cmp.Diff(hrp1Result, hrpReady1)).To(BeEmpty())

				hrp2Result := &addonsv1alpha1.HelmReleaseProxy{}
				g.Expect(c.Get(ctx, client.ObjectKey{Namespace: hrpReady2.Namespace, Name: hrpReady2.Name}, hrp2Result)).To(Succeed())
				g.Expect(cmp.Diff(hrp2Result, hrpReady2)).To(BeEmpty())

				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
			},
			expectedError: "",
		},
		{
			name:           "mark HelmChartProxy as ready once HelmReleaseProxies ready conditions are true ignoring paused clusters",
			helmChartProxy: continuousProxy,
			objects:        []client.Object{cluster1, cluster2, clusterPaused, hrpReady1, hrpReady2},
			expect: func(g *WithT, c client.Client, hcp *addonsv1alpha1.HelmChartProxy) {
				g.Expect(hcp.Status.MatchingClusters).To(BeEquivalentTo([]corev1.ObjectReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-1",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-2",
						Namespace:  "test-namespace",
					},
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster-paused",
						Namespace:  "test-namespace",
					},
				}))
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, addonsv1alpha1.HelmReleaseProxiesReadyCondition)).To(BeTrue())
				g.Expect(conditions.Has(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hcp, clusterv1.ReadyCondition)).To(BeTrue())
				g.Expect(hcp.Status.ObservedGeneration).To(Equal(hcp.Generation))
				hrpList := &addonsv1alpha1.HelmReleaseProxyList{}
				g.Expect(c.List(ctx, hrpList, &client.ListOptions{Namespace: hcp.Namespace})).To(Succeed())

				// There should be 2 HelmReleaseProxies as the paused cluster should not have a HelmReleaseProxy.
				g.Expect(hrpList.Items).To(HaveLen(2))
			},
			expectedError: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			request := reconcile.Request{
				NamespacedName: util.ObjectKey(tc.helmChartProxy),
			}

			tc.objects = append(tc.objects, tc.helmChartProxy)
			r := &HelmChartProxyReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(fakeScheme).
					WithObjects(tc.objects...).
					WithStatusSubresource(&addonsv1alpha1.HelmChartProxy{}).
					WithStatusSubresource(&addonsv1alpha1.HelmReleaseProxy{}).
					Build(),
			}
			result, err := r.Reconcile(ctx, request)

			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(reconcile.Result{}))

				hcp := &addonsv1alpha1.HelmChartProxy{}
				g.Expect(r.Client.Get(ctx, request.NamespacedName, hcp)).To(Succeed())

				tc.expect(g, r.Client, hcp)
			}
		})
	}
}

func TestReconcileAfterMatchingClusterUnpaused(t *testing.T) {
	g := NewWithT(t)

	request := reconcile.Request{
		NamespacedName: util.ObjectKey(continuousProxy),
	}

	c := fake.NewClientBuilder().
		WithScheme(fakeScheme).
		WithObjects(cluster1, cluster2, clusterPaused, continuousProxy).
		WithStatusSubresource(&addonsv1alpha1.HelmChartProxy{}).
		WithStatusSubresource(&addonsv1alpha1.HelmReleaseProxy{}).
		Build()

	r := &HelmChartProxyReconciler{
		Client: c,
	}
	result, err := r.Reconcile(ctx, request)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(reconcile.Result{}))

	// There should be 2 HelmReleaseProxies as the paused cluster should not have a HelmReleaseProxy.
	hrpList := &addonsv1alpha1.HelmReleaseProxyList{}
	g.Expect(c.List(ctx, hrpList, &client.ListOptions{Namespace: request.Namespace})).To(Succeed())
	g.Expect(hrpList.Items).To(HaveLen(2))

	// Unpause the cluster and reconcile again.
	unpausedCluster := clusterPaused.DeepCopy()
	unpausedCluster.Spec.Paused = false
	g.Expect(c.Update(ctx, unpausedCluster)).To(Succeed())

	result, err = r.Reconcile(ctx, request)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(reconcile.Result{}))

	// Now there should be 3 HelmReleaseProxies.
	hrpList = &addonsv1alpha1.HelmReleaseProxyList{}
	g.Expect(c.List(ctx, hrpList, &client.ListOptions{Namespace: request.Namespace})).To(Succeed())
	g.Expect(hrpList.Items).To(HaveLen(3))
}

func init() {
	_ = scheme.AddToScheme(fakeScheme)
	_ = clusterv1.AddToScheme(fakeScheme)
	_ = addonsv1alpha1.AddToScheme(fakeScheme)
}
