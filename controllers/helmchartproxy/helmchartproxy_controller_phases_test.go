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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = &HelmChartProxyReconciler{}

var (
	fakeHelmChartProxy1 = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ReleaseName:      "test-release-name",
			ChartName:        "test-chart-name",
			RepoURL:          "https://test-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			ValuesTemplate:   "apiServerPort: {{ .Cluster.spec.clusterNetwork.apiServerPort }}",
			Options: addonsv1alpha1.HelmOptions{
				EnableClientCache: true,
				Timeout: &metav1.Duration{
					Duration: 10 * time.Minute,
				},
			},
		},
	}

	fakeHelmChartProxy2 = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ReleaseName:      "test-release-name",
			ChartName:        "test-chart-name",
			RepoURL:          "https://test-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			ValuesTemplate:   "cidrBlockList: {{ .Cluster.spec.clusterNetwork.pods.cidrBlocks | join \",\" }}",
			Options: addonsv1alpha1.HelmOptions{
				EnableClientCache: true,
				Timeout: &metav1.Duration{
					Duration: 10 * time.Minute,
				},
			},
		},
	}

	fakeInvalidHelmChartProxy = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ReleaseName:      "test-release-name",
			ChartName:        "test-chart-name",
			RepoURL:          "https://test-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			ValuesTemplate:   "apiServerPort: {{ .Cluster.invalid-path }}",
			Options: addonsv1alpha1.HelmOptions{
				EnableClientCache: true,
				Timeout: &metav1.Duration{
					Duration: 10 * time.Minute,
				},
			},
		},
	}

	fakeReinstallHelmChartProxy = &addonsv1alpha1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: addonsv1alpha1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcp",
			Namespace: "test-namespace",
		},
		Spec: addonsv1alpha1.HelmChartProxySpec{
			ReleaseName:      "other-release-name",
			ChartName:        "other-chart-name",
			RepoURL:          "https://other-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			ValuesTemplate:   "apiServerPort: {{ .Cluster.spec.clusterNetwork.apiServerPort }}",
			Options: addonsv1alpha1.HelmOptions{
				EnableClientCache: true,
				Timeout: &metav1.Duration{
					Duration: 10 * time.Minute,
				},
			},
		},
	}

	fakeCluster1 = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				APIServerPort: ptr.To(int32(6443)),
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{"10.0.0.0/16", "20.0.0.0/16"},
				},
			},
		},
	}

	fakeCluster2 = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				APIServerPort: ptr.To(int32(1234)),
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{"10.0.0.0/16", "20.0.0.0/16"},
				},
			},
		},
	}

	fakeClusterPaused = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				APIServerPort: ptr.To(int32(1234)),
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{"10.0.0.0/16", "20.0.0.0/16"},
				},
			},
			Paused: true,
		},
	}

	fakeHelmReleaseProxy = &addonsv1alpha1.HelmReleaseProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-generated-name",
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
				clusterv1.ClusterNameLabel:             "test-cluster",
				addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
			},
		},
		Spec: addonsv1alpha1.HelmReleaseProxySpec{
			ClusterRef: corev1.ObjectReference{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
				Name:       "test-cluster",
				Namespace:  "test-namespace",
			},
			ReleaseName:      "test-release-name",
			ChartName:        "test-chart-name",
			RepoURL:          "https://test-repo-url",
			ReleaseNamespace: "test-release-namespace",
			Version:          "test-version",
			Values:           "apiServerPort: 6443",
			Options: addonsv1alpha1.HelmOptions{
				EnableClientCache: true,
				Timeout: &metav1.Duration{
					Duration: 10 * time.Minute,
				},
			},
		},
	}
)

func TestReconcileForCluster(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name                          string
		helmChartProxy                *addonsv1alpha1.HelmChartProxy
		existingHelmReleaseProxy      *addonsv1alpha1.HelmReleaseProxy
		cluster                       *clusterv1.Cluster
		expect                        func(g *WithT, hcp *addonsv1alpha1.HelmChartProxy, hrp *addonsv1alpha1.HelmReleaseProxy)
		expectHelmReleaseProxyToExist bool
		expectedError                 string
	}{
		{
			name:                          "creates a HelmReleaseProxy for a HelmChartProxy",
			helmChartProxy:                fakeHelmChartProxy1,
			cluster:                       fakeCluster1,
			expectHelmReleaseProxyToExist: true,
			expect: func(g *WithT, hcp *addonsv1alpha1.HelmChartProxy, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(hrp.Spec.Values).To(Equal("apiServerPort: 6443"))
			},
			expectedError: "",
		},
		{
			name:                          "updates a HelmReleaseProxy when Cluster value changes",
			helmChartProxy:                fakeHelmChartProxy1,
			existingHelmReleaseProxy:      fakeHelmReleaseProxy,
			cluster:                       fakeCluster2,
			expectHelmReleaseProxyToExist: true,
			expect: func(g *WithT, hcp *addonsv1alpha1.HelmChartProxy, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(hrp.Spec.Values).To(Equal("apiServerPort: 1234"))
			},
			expectedError: "",
		},
		{
			name:                          "updates a HelmReleaseProxy when valuesTemplate value changes",
			helmChartProxy:                fakeHelmChartProxy2,
			existingHelmReleaseProxy:      fakeHelmReleaseProxy,
			cluster:                       fakeCluster2,
			expectHelmReleaseProxyToExist: true,
			expect: func(g *WithT, hcp *addonsv1alpha1.HelmChartProxy, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(hrp.Spec.Values).To(Equal("cidrBlockList: 10.0.0.0/16,20.0.0.0/16"))
			},
			expectedError: "",
		},
		{
			name:                          "set condition when failing to parse values for a HelmChartProxy",
			helmChartProxy:                fakeInvalidHelmChartProxy,
			cluster:                       fakeCluster1,
			expectHelmReleaseProxyToExist: true,
			expect: func(g *WithT, hcp *addonsv1alpha1.HelmChartProxy, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				specsReady := conditions.Get(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)
				g.Expect(specsReady.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(specsReady.Reason).To(Equal(addonsv1alpha1.ValueParsingFailedReason))
				g.Expect(specsReady.Severity).To(Equal(clusterv1.ConditionSeverityError))
				g.Expect(specsReady.Message).To(Equal("failed to parse values on cluster test-cluster: template: test-chart-name-test-cluster:1: bad character U+002D '-'"))
			},
			expectedError: "failed to parse values on cluster test-cluster: template: test-chart-name-test-cluster:1: bad character U+002D '-'",
		},
		{
			name:                          "set condition for reinstalling when requeueing after a deletion",
			helmChartProxy:                fakeReinstallHelmChartProxy,
			existingHelmReleaseProxy:      fakeHelmReleaseProxy,
			cluster:                       fakeCluster1,
			expectHelmReleaseProxyToExist: false,
			expect: func(g *WithT, hcp *addonsv1alpha1.HelmChartProxy, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeTrue())
				specsReady := conditions.Get(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)
				g.Expect(specsReady.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(specsReady.Reason).To(Equal(addonsv1alpha1.HelmReleaseProxyReinstallingReason))
				g.Expect(specsReady.Severity).To(Equal(clusterv1.ConditionSeverityInfo))
				g.Expect(specsReady.Message).To(Equal(fmt.Sprintf("HelmReleaseProxy on cluster '%s' successfully deleted, preparing to reinstall", fakeCluster1.Name)))
			},
			expectedError: "",
		},
		{
			name:                          "do not reconcile for a paused cluster",
			helmChartProxy:                fakeHelmChartProxy1,
			existingHelmReleaseProxy:      fakeHelmReleaseProxy,
			cluster:                       fakeClusterPaused,
			expectHelmReleaseProxyToExist: false,
			expect: func(g *WithT, hcp *addonsv1alpha1.HelmChartProxy, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(conditions.Has(hcp, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)).To(BeFalse())
			},
			expectedError: "",
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			objects := []client.Object{tc.helmChartProxy, tc.cluster}
			if tc.existingHelmReleaseProxy != nil {
				objects = append(objects, tc.existingHelmReleaseProxy)
			}
			r := &HelmChartProxyReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(fakeScheme).
					WithObjects(objects...).
					WithStatusSubresource(&addonsv1alpha1.HelmChartProxy{}).
					WithStatusSubresource(&addonsv1alpha1.HelmReleaseProxy{}).
					Build(),
			}
			err := r.reconcileForCluster(ctx, tc.helmChartProxy, *tc.cluster)

			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				var hrp *addonsv1alpha1.HelmReleaseProxy
				var err error
				if tc.expectHelmReleaseProxyToExist {
					hrp, err = r.getExistingHelmReleaseProxy(ctx, tc.helmChartProxy, tc.cluster)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(hrp).NotTo(BeNil())
				}
				tc.expect(g, tc.helmChartProxy, hrp)
			}
		})
	}
}

func TestConstructHelmReleaseProxy(t *testing.T) {
	testCases := []struct {
		name           string
		existing       *addonsv1alpha1.HelmReleaseProxy
		helmChartProxy *addonsv1alpha1.HelmChartProxy
		parsedValues   string
		cluster        *clusterv1.Cluster
		expected       *addonsv1alpha1.HelmReleaseProxy
	}{
		{
			name: "existing up to date, nothing to do",
			existing: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-generated-name",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
						Namespace:  "test-namespace",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "test-parsed-values",
					Version:          "test-version",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: addonsv1alpha1.GroupVersion.String(),
					Kind:       "HelmChartProxy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hcp",
					Namespace: "test-namespace",
				},
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Version:          "test-version",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
			parsedValues: "test-parsed-values",
			cluster: &clusterv1.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			expected: nil,
		},
		{
			name:     "construct helm release proxy without existing",
			existing: nil,
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: addonsv1alpha1.GroupVersion.String(),
					Kind:       "HelmChartProxy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hcp",
					Namespace: "test-namespace",
				},
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
			parsedValues: "test-parsed-values",
			cluster: &clusterv1.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			expected: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-chart-name-test-cluster-",
					Namespace:    "test-namespace",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
						Namespace:  "test-namespace",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "test-parsed-values",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
		},
		{
			name: "version changed",
			existing: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-generated-name",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
						Namespace:  "test-namespace",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "test-parsed-values",
					Version:          "test-version",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: addonsv1alpha1.GroupVersion.String(),
					Kind:       "HelmChartProxy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hcp",
					Namespace: "test-namespace",
				},
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Version:          "another-version",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
			parsedValues: "test-parsed-values",
			cluster: &clusterv1.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			expected: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-generated-name",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
						Namespace:  "test-namespace",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "test-parsed-values",
					Version:          "another-version",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
		},
		{
			name: "parsed values changed",
			existing: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-generated-name",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
						Namespace:  "test-namespace",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "test-parsed-values",
					Version:          "test-version",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: addonsv1alpha1.GroupVersion.String(),
					Kind:       "HelmChartProxy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hcp",
					Namespace: "test-namespace",
				},
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Version:          "test-version",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
			parsedValues: "updated-parsed-values",
			cluster: &clusterv1.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			expected: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-generated-name",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
						Namespace:  "test-namespace",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "updated-parsed-values",
					Version:          "test-version",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
				},
			},
		},
		{
			name:     "construct helm release proxy with secret",
			existing: nil,
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: addonsv1alpha1.GroupVersion.String(),
					Kind:       "HelmChartProxy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hcp",
					Namespace: "test-namespace",
				},
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
					Credentials: &addonsv1alpha1.Credentials{
						Secret: corev1.SecretReference{
							Name: "test-secret",
						},
					},
				},
			},
			parsedValues: "test-parsed-values",
			cluster: &clusterv1.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expected: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-chart-name-test-cluster-",
					Namespace:    "test-namespace",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "test-parsed-values",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
					Credentials: &addonsv1alpha1.Credentials{
						Secret: corev1.SecretReference{
							Name:      "test-secret",
							Namespace: "test-namespace",
						},
						Key: "config.json",
					},
				},
			},
		},
		{
			name:     "construct helm release proxy with secret and custom namespace",
			existing: nil,
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: addonsv1alpha1.GroupVersion.String(),
					Kind:       "HelmChartProxy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hcp",
					Namespace: "test-namespace",
				},
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
					Credentials: &addonsv1alpha1.Credentials{
						Secret: corev1.SecretReference{
							Name:      "test-secret",
							Namespace: "my-namespace",
						},
					},
				},
			},
			parsedValues: "test-parsed-values",
			cluster: &clusterv1.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expected: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-chart-name-test-cluster-",
					Namespace:    "test-namespace",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "test-parsed-values",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
					Credentials: &addonsv1alpha1.Credentials{
						Secret: corev1.SecretReference{
							Name:      "test-secret",
							Namespace: "my-namespace",
						},
						Key: "config.json",
					},
				},
			},
		},
		{
			name:     "construct helm release proxy with secret and custom key",
			existing: nil,
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: addonsv1alpha1.GroupVersion.String(),
					Kind:       "HelmChartProxy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hcp",
					Namespace: "test-namespace",
				},
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
					Credentials: &addonsv1alpha1.Credentials{
						Secret: corev1.SecretReference{
							Name: "test-secret",
						},
						Key: "custom-key.json",
					},
				},
			},
			parsedValues: "test-parsed-values",
			cluster: &clusterv1.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expected: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-chart-name-test-cluster-",
					Namespace:    "test-namespace",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "test-parsed-values",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
					Credentials: &addonsv1alpha1.Credentials{
						Secret: corev1.SecretReference{
							Name:      "test-secret",
							Namespace: "test-namespace",
						},
						Key: "custom-key.json",
					},
				},
			},
		},
		{
			name:     "construct helm release proxy with insecure TLS",
			existing: nil,
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: addonsv1alpha1.GroupVersion.String(),
					Kind:       "HelmChartProxy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hcp",
					Namespace: "test-namespace",
				},
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
					TLSConfig: &addonsv1alpha1.TLSConfig{
						InsecureSkipTLSVerify: true,
					},
				},
			},
			parsedValues: "test-parsed-values",
			cluster: &clusterv1.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expected: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-chart-name-test-cluster-",
					Namespace:    "test-namespace",
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
						clusterv1.ClusterNameLabel:             "test-cluster",
						addonsv1alpha1.HelmChartProxyLabelName: "test-hcp",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ClusterRef: corev1.ObjectReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "test-cluster",
					},
					ReleaseName:      "test-release-name",
					ChartName:        "test-chart-name",
					RepoURL:          "https://test-repo-url",
					ReleaseNamespace: "test-release-namespace",
					Values:           "test-parsed-values",
					Options: addonsv1alpha1.HelmOptions{
						EnableClientCache: true,
						Timeout: &metav1.Duration{
							Duration: 10 * time.Minute,
						},
					},
					TLSConfig: &addonsv1alpha1.TLSConfig{
						InsecureSkipTLSVerify: true,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result := constructHelmReleaseProxy(tc.existing, tc.helmChartProxy, tc.parsedValues, tc.cluster)
			diff := cmp.Diff(tc.expected, result)
			g.Expect(diff).To(BeEmpty())
		})
	}
}

func TestShouldReinstallHelmRelease(t *testing.T) {
	testCases := []struct {
		name             string
		helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
		helmChartProxy   *addonsv1alpha1.HelmChartProxy
		reinstall        bool
	}{
		{
			name: "nothing to do",
			helmReleaseProxy: &addonsv1alpha1.HelmReleaseProxy{
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ChartName:        "test-chart",
					RepoURL:          "https://test-repo-url",
					ReleaseName:      "test-release-name",
					ReleaseNamespace: "test-namespace",
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ChartName:        "test-chart",
					RepoURL:          "https://test-repo-url",
					ReleaseName:      "test-release-name",
					ReleaseNamespace: "test-namespace",
				},
			},
			reinstall: false,
		},
		{
			name: "chart name changed, should reinstall",
			helmReleaseProxy: &addonsv1alpha1.HelmReleaseProxy{
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ChartName:        "test-chart",
					RepoURL:          "https://test-repo-url",
					ReleaseName:      "test-release-name",
					ReleaseNamespace: "test-namespace",
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ChartName:        "another-chart",
					RepoURL:          "https://test-repo-url",
					ReleaseName:      "test-release-name",
					ReleaseNamespace: "test-namespace",
				},
			},
			reinstall: true,
		},
		{
			name: "repo url changed, should reinstall",
			helmReleaseProxy: &addonsv1alpha1.HelmReleaseProxy{
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ChartName:        "test-chart",
					RepoURL:          "https://test-repo-url",
					ReleaseName:      "test-release-name",
					ReleaseNamespace: "test-namespace",
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ChartName:        "test-chart",
					RepoURL:          "https://another-repo-url",
					ReleaseName:      "test-release-name",
					ReleaseNamespace: "test-namespace",
				},
			},
			reinstall: true,
		},
		{
			name: "generated release name changed, should reinstall",
			helmReleaseProxy: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						addonsv1alpha1.IsReleaseNameGeneratedAnnotation: "true",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ChartName:   "test-chart",
					RepoURL:     "https://test-repo-url",
					ReleaseName: "generated-release-name",
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ChartName:   "test-chart",
					RepoURL:     "https://test-repo-url",
					ReleaseName: "some-other-release-name",
				},
			},
			reinstall: true,
		},
		{
			name: "generated release name unchanged, nothing to do",
			helmReleaseProxy: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						addonsv1alpha1.IsReleaseNameGeneratedAnnotation: "true",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ChartName:   "test-chart",
					RepoURL:     "https://test-repo-url",
					ReleaseName: "generated-release-name",
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ChartName:   "test-chart",
					RepoURL:     "https://test-repo-url",
					ReleaseName: "",
				},
			},
			reinstall: false,
		},
		{
			name: "non-generated release name changed, should reinstall",
			helmReleaseProxy: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						addonsv1alpha1.IsReleaseNameGeneratedAnnotation: "true",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ChartName:   "test-chart",
					RepoURL:     "https://test-repo-url",
					ReleaseName: "test-release-name",
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ChartName:   "test-chart",
					RepoURL:     "https://test-repo-url",
					ReleaseName: "some-other-release-name",
				},
			},
			reinstall: true,
		},
		{
			name: "release namespace changed, should reinstall",
			helmReleaseProxy: &addonsv1alpha1.HelmReleaseProxy{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						addonsv1alpha1.IsReleaseNameGeneratedAnnotation: "true",
					},
				},
				Spec: addonsv1alpha1.HelmReleaseProxySpec{
					ChartName:        "test-chart",
					RepoURL:          "https://test-repo-url",
					ReleaseName:      "test-release-name",
					ReleaseNamespace: "test-namespace",
				},
			},
			helmChartProxy: &addonsv1alpha1.HelmChartProxy{
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ChartName:        "test-chart",
					RepoURL:          "https://test-repo-url",
					ReleaseName:      "test-release-name",
					ReleaseNamespace: "some-other-namespace",
				},
			},
			reinstall: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result := shouldReinstallHelmRelease(ctx, tc.helmReleaseProxy, tc.helmChartProxy)
			g.Expect(result).To(Equal(tc.reinstall))
		})
	}
}

func TestGetOrphanedHelmReleaseProxies(t *testing.T) {
	testCases := []struct {
		name               string
		selectedClusters   []clusterv1.Cluster
		helmReleaseProxies []addonsv1alpha1.HelmReleaseProxy
		releasesToDelete   []addonsv1alpha1.HelmReleaseProxy
	}{
		{
			name: "nothing to do",
			selectedClusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-1",
						Namespace: "test-namespace-1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-2",
						Namespace: "test-namespace-2",
					},
				},
			},
			helmReleaseProxies: []addonsv1alpha1.HelmReleaseProxy{
				{
					Spec: addonsv1alpha1.HelmReleaseProxySpec{
						ClusterRef: corev1.ObjectReference{
							Name:      "test-cluster-1",
							Namespace: "test-namespace-1",
						},
					},
				},
				{
					Spec: addonsv1alpha1.HelmReleaseProxySpec{
						ClusterRef: corev1.ObjectReference{
							Name:      "test-cluster-2",
							Namespace: "test-namespace-2",
						},
					},
				},
			},
			releasesToDelete: []addonsv1alpha1.HelmReleaseProxy{},
		},
		{
			name: "delete one release",
			selectedClusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-1",
						Namespace: "test-namespace-1",
					},
				},
			},
			helmReleaseProxies: []addonsv1alpha1.HelmReleaseProxy{
				{
					Spec: addonsv1alpha1.HelmReleaseProxySpec{
						ClusterRef: corev1.ObjectReference{
							Name:      "test-cluster-1",
							Namespace: "test-namespace-1",
						},
					},
				},
				{
					Spec: addonsv1alpha1.HelmReleaseProxySpec{
						ClusterRef: corev1.ObjectReference{
							Name:      "test-cluster-2",
							Namespace: "test-namespace-2",
						},
					},
				},
			},
			releasesToDelete: []addonsv1alpha1.HelmReleaseProxy{
				{
					Spec: addonsv1alpha1.HelmReleaseProxySpec{
						ClusterRef: corev1.ObjectReference{
							Name:      "test-cluster-2",
							Namespace: "test-namespace-2",
						},
					},
				},
			},
		},
		{
			name:             "delete both releases",
			selectedClusters: []clusterv1.Cluster{},
			helmReleaseProxies: []addonsv1alpha1.HelmReleaseProxy{
				{
					Spec: addonsv1alpha1.HelmReleaseProxySpec{
						ClusterRef: corev1.ObjectReference{
							Name:      "test-cluster-1",
							Namespace: "test-namespace-1",
						},
					},
				},
				{
					Spec: addonsv1alpha1.HelmReleaseProxySpec{
						ClusterRef: corev1.ObjectReference{
							Name:      "test-cluster-2",
							Namespace: "test-namespace-2",
						},
					},
				},
			},
			releasesToDelete: []addonsv1alpha1.HelmReleaseProxy{
				{
					Spec: addonsv1alpha1.HelmReleaseProxySpec{
						ClusterRef: corev1.ObjectReference{
							Name:      "test-cluster-1",
							Namespace: "test-namespace-1",
						},
					},
				},
				{
					Spec: addonsv1alpha1.HelmReleaseProxySpec{
						ClusterRef: corev1.ObjectReference{
							Name:      "test-cluster-2",
							Namespace: "test-namespace-2",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result := getOrphanedHelmReleaseProxies(ctx, tc.selectedClusters, tc.helmReleaseProxies)
			g.Expect(result).To(Equal(tc.releasesToDelete))
		})
	}
}
