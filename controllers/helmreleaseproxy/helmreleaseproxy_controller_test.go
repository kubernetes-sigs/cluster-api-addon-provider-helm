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

package helmreleaseproxy

import (
	"fmt"
	"testing"

	helmRelease "helm.sh/helm/v3/pkg/release"
	helmDriver "helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"sigs.k8s.io/cluster-api-addon-provider-helm/internal/mocks"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
)

var (
	kubeconfig = "test-kubeconfig"

	defaultProxy = &addonsv1alpha1.HelmReleaseProxy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmReleaseProxy",
			APIVersion: "addons.cluster.x-k8s.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-proxy",
			Namespace: "default",
		},
		Spec: addonsv1alpha1.HelmReleaseProxySpec{
			ClusterRef: corev1.ObjectReference{
				APIVersion: "cluster.x-k8s.io/v1beta1",
				Kind:       "Cluster",
				Namespace:  "default",
				Name:       "test-cluster",
			},
			RepoURL:          "https://test-repo",
			ChartName:        "test-chart",
			Version:          "test-version",
			ReleaseName:      "test-release",
			ReleaseNamespace: "default",
			Values:           "test-values",
		},
	}

	generateNameProxy = &addonsv1alpha1.HelmReleaseProxy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmReleaseProxy",
			APIVersion: "addons.cluster.x-k8s.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-proxy",
			Namespace: "default",
		},
		Spec: addonsv1alpha1.HelmReleaseProxySpec{
			ClusterRef: corev1.ObjectReference{
				APIVersion: "cluster.x-k8s.io/v1beta1",
				Kind:       "Cluster",
				Namespace:  "default",
				Name:       "test-cluster",
			},
			RepoURL:          "https://test-repo",
			ChartName:        "test-chart",
			Version:          "test-version",
			ReleaseNamespace: "default",
			Values:           "test-values",
		},
	}

	errInternal = fmt.Errorf("internal error")
)

func TestReconcileNormal(t *testing.T) {
	testcases := []struct {
		name             string
		helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
		clientExpect     func(g *WithT, c *mocks.MockClientMockRecorder)
		expect           func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy)
		expectedError    string
	}{
		{
			name:             "succesfully install a Helm release",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, kubeconfig, defaultProxy.DeepCopy().Spec).Return(&helmRelease.Release{
					Name:    "test-release",
					Version: 1,
					Info: &helmRelease.Info{
						Status: helmRelease.StatusDeployed,
					},
				}, nil).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				_, ok := hrp.Annotations[addonsv1alpha1.IsReleaseNameGeneratedAnnotation]
				g.Expect(ok).To(BeFalse())
				g.Expect(hrp.Spec.ReleaseName).To(Equal("test-release"))
				g.Expect(hrp.Status.Revision).To(Equal(1))
				g.Expect(hrp.Status.Status).To(BeEquivalentTo(helmRelease.StatusDeployed))

				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())

			},
			expectedError: "",
		},
		{
			name:             "succesfully install a Helm release with a generated name",
			helmReleaseProxy: generateNameProxy,
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, kubeconfig, generateNameProxy.Spec).Return(&helmRelease.Release{
					Name:    "test-release",
					Version: 1,
					Info: &helmRelease.Info{
						Status: helmRelease.StatusDeployed,
					},
				}, nil).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				_, ok := hrp.Annotations[addonsv1alpha1.IsReleaseNameGeneratedAnnotation]
				g.Expect(ok).To(BeTrue())
				g.Expect(hrp.Spec.ReleaseName).To(Equal("test-release"))
				g.Expect(hrp.Status.Revision).To(Equal(1))
				g.Expect(hrp.Status.Status).To(BeEquivalentTo(helmRelease.StatusDeployed))

				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
			},
			expectedError: "",
		},
		{
			name:             "Helm release pending",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, kubeconfig, defaultProxy.Spec).Return(&helmRelease.Release{
					Name:    "test-release",
					Version: 1,
					Info: &helmRelease.Info{
						Status: helmRelease.StatusPendingInstall,
					},
				}, nil).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				t.Logf("HelmReleaseProxy: %+v", hrp)
				_, ok := hrp.Annotations[addonsv1alpha1.IsReleaseNameGeneratedAnnotation]
				g.Expect(ok).To(BeFalse())
				g.Expect(hrp.Spec.ReleaseName).To(Equal("test-release"))
				g.Expect(hrp.Status.Revision).To(Equal(1))
				g.Expect(hrp.Status.Status).To(BeEquivalentTo(helmRelease.StatusPendingInstall))

				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmReleasePendingReason))
				g.Expect(releaseReady.Severity).To(Equal(clusterv1.ConditionSeverityInfo))
			},
			expectedError: "",
		},
		{
			name:             "Helm client returns error",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, kubeconfig, defaultProxy.Spec).Return(nil, errInternal).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				_, ok := hrp.Annotations[addonsv1alpha1.IsReleaseNameGeneratedAnnotation]
				g.Expect(ok).To(BeFalse())

				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmInstallOrUpgradeFailedReason))
				g.Expect(releaseReady.Severity).To(Equal(clusterv1.ConditionSeverityError))
				g.Expect(releaseReady.Message).To(Equal(errInternal.Error()))

			},
			expectedError: errInternal.Error(),
		},
		{
			name:             "Helm release in a failed state, no client error",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, kubeconfig, defaultProxy.Spec).Return(&helmRelease.Release{
					Name:    "test-release",
					Version: 1,
					Info: &helmRelease.Info{
						Status: helmRelease.StatusFailed,
					},
				}, nil).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				_, ok := hrp.Annotations[addonsv1alpha1.IsReleaseNameGeneratedAnnotation]
				g.Expect(ok).To(BeFalse())

				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmInstallOrUpgradeFailedReason))
				g.Expect(releaseReady.Severity).To(Equal(clusterv1.ConditionSeverityError))
				g.Expect(releaseReady.Message).To(Equal(fmt.Sprintf("Helm release failed: %s", helmRelease.StatusFailed)))

			},
			expectedError: "",
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			clientMock := mocks.NewMockClient(mockCtrl)
			tc.clientExpect(g, clientMock.EXPECT())

			r := &HelmReleaseProxyReconciler{
				Client: fake.NewClientBuilder().WithScheme(fakeScheme).Build(),
			}

			err := r.reconcileNormal(ctx, tc.helmReleaseProxy, clientMock, kubeconfig)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				tc.expect(g, tc.helmReleaseProxy)
			}
		})
	}
}

func TestReconcileDelete(t *testing.T) {
	testcases := []struct {
		name             string
		helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
		clientExpect     func(g *WithT, c *mocks.MockClientMockRecorder)
		expect           func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy)
		expectedError    string
	}{
		{
			name:             "succesfully uninstall a Helm release",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.GetHelmRelease(ctx, kubeconfig, defaultProxy.DeepCopy().Spec).Return(&helmRelease.Release{
					Name:    "test-release",
					Version: 1,
					Info: &helmRelease.Info{
						Status: helmRelease.StatusDeployed,
					},
				}, nil).Times(1)
				c.UninstallHelmRelease(ctx, kubeconfig, defaultProxy.DeepCopy().Spec).Return(&helmRelease.UninstallReleaseResponse{}, nil).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmReleaseDeletedReason))
				g.Expect(releaseReady.Severity).To(Equal(clusterv1.ConditionSeverityInfo))
			},
			expectedError: "",
		},
		{
			name:             "Helm release already uninstalled",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.GetHelmRelease(ctx, kubeconfig, defaultProxy.DeepCopy().Spec).Return(nil, helmDriver.ErrReleaseNotFound).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmReleaseDeletedReason))
				g.Expect(releaseReady.Severity).To(Equal(clusterv1.ConditionSeverityInfo))
			},
			expectedError: "",
		},
		{
			name:             "error attempting to get Helm release",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.GetHelmRelease(ctx, kubeconfig, defaultProxy.DeepCopy().Spec).Return(nil, errInternal).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmReleaseReadyCondition))
				g.Expect(releaseReady.Severity).To(Equal(clusterv1.ConditionSeverityError))
			},
			expectedError: errInternal.Error(),
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			clientMock := mocks.NewMockClient(mockCtrl)
			tc.clientExpect(g, clientMock.EXPECT())

			r := &HelmReleaseProxyReconciler{
				Client: fake.NewClientBuilder().WithScheme(fakeScheme).Build(),
			}

			err := r.reconcileDelete(ctx, tc.helmReleaseProxy, clientMock, kubeconfig)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				tc.expect(g, tc.helmReleaseProxy)
			}
		})
	}
}
