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

	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	helmRelease "helm.sh/helm/v3/pkg/release"
	helmDriver "helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"sigs.k8s.io/cluster-api-addon-provider-helm/internal/mocks"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	ctx        = ctrl.SetupSignalHandler()
	fakeScheme = runtime.NewScheme()

	restConfig = &rest.Config{}

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
			RepoURL:           "https://test-repo",
			ChartName:         "test-chart",
			Version:           "test-version",
			ReleaseName:       "test-release",
			ReleaseNamespace:  "default",
			Values:            "test-values",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyContinuous),
			Credentials:       nil,
		},
	}

	installOnceProxyAlreadyInstalled = &addonsv1alpha1.HelmReleaseProxy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmReleaseProxy",
			APIVersion: "addons.cluster.x-k8s.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-proxy",
			Namespace: "default",
			Annotations: map[string]string{
				addonsv1alpha1.ReleaseSuccessfullyInstalledAnnotation: "true",
			},
		},
		Spec: addonsv1alpha1.HelmReleaseProxySpec{
			ClusterRef: corev1.ObjectReference{
				APIVersion: "cluster.x-k8s.io/v1beta1",
				Kind:       "Cluster",
				Namespace:  "default",
				Name:       "test-cluster",
			},
			RepoURL:           "https://test-repo",
			ChartName:         "test-chart",
			Version:           "test-version",
			ReleaseName:       "test-release",
			ReleaseNamespace:  "default",
			Values:            "test-values",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyInstallOnce),
			Credentials:       nil,
		},
		Status: addonsv1alpha1.HelmReleaseProxyStatus{
			Status:   string(helmRelease.StatusDeployed),
			Revision: 1,
			Conditions: []metav1.Condition{
				{
					Type:   addonsv1alpha1.HelmReleaseReadyCondition,
					Status: metav1.ConditionTrue,
				},
				{
					Type:   addonsv1alpha1.ClusterAvailableCondition,
					Status: metav1.ConditionTrue,
				},
				{
					Type:   clusterv1.ReadyCondition,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}

	installOnceProxyNotInstalled = &addonsv1alpha1.HelmReleaseProxy{
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
			RepoURL:           "https://test-repo",
			ChartName:         "test-chart",
			Version:           "test-version",
			ReleaseName:       "test-release",
			ReleaseNamespace:  "default",
			Values:            "test-values",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyInstallOnce),
			Credentials:       nil,
		},
	}

	defaultProxyWithCredentialRef = &addonsv1alpha1.HelmReleaseProxy{
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
			RepoURL:           "https://test-repo",
			ChartName:         "test-chart",
			Version:           "test-version",
			ReleaseName:       "test-release",
			ReleaseNamespace:  "default",
			Values:            "test-values",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyContinuous),
			Credentials: &addonsv1alpha1.Credentials{
				Secret: corev1.SecretReference{
					Name:      "test-secret",
					Namespace: "default",
				},
			},
		},
	}

	defaultProxyWithCACertRef = &addonsv1alpha1.HelmReleaseProxy{
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
			RepoURL:           "https://test-repo",
			ChartName:         "test-chart",
			Version:           "test-version",
			ReleaseName:       "test-release",
			ReleaseNamespace:  "default",
			Values:            "test-values",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyContinuous),
			TLSConfig: &addonsv1alpha1.TLSConfig{
				CASecretRef: &corev1.SecretReference{
					Name:      "test-secret",
					Namespace: "default",
				},
			},
		},
	}

	defaultProxyWithSkipTLS = &addonsv1alpha1.HelmReleaseProxy{
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
			RepoURL:           "https://test-repo",
			ChartName:         "test-chart",
			Version:           "test-version",
			ReleaseName:       "test-release",
			ReleaseNamespace:  "default",
			Values:            "test-values",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyContinuous),
			TLSConfig: &addonsv1alpha1.TLSConfig{
				InsecureSkipTLSVerify: true,
			},
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
			RepoURL:           "https://test-repo",
			ChartName:         "test-chart",
			Version:           "test-version",
			ReleaseNamespace:  "default",
			Values:            "test-values",
			ReconcileStrategy: string(addonsv1alpha1.ReconcileStrategyContinuous),
		},
	}

	errInternal = fmt.Errorf("internal error")
)

func TestReconcileNormal(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name             string
		helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
		clientExpect     func(g *WithT, c *mocks.MockClientMockRecorder)
		expect           func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy)
		expectedError    string
	}{
		{
			name:             "successfully install a Helm release",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, restConfig, "", "", defaultProxy.DeepCopy().Spec).Return(&helmRelease.Release{
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
			name:             "successfully install a Helm release with a generated name",
			helmReleaseProxy: generateNameProxy,
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, restConfig, "", "", generateNameProxy.Spec).Return(&helmRelease.Release{
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
				c.InstallOrUpgradeHelmRelease(ctx, restConfig, "", "", defaultProxy.Spec).Return(&helmRelease.Release{
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
				g.Expect(releaseReady.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmReleasePendingReason))
			},
			expectedError: "",
		},
		{
			name:             "Helm client returns error",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, restConfig, "", "", defaultProxy.Spec).Return(nil, errInternal).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				_, ok := hrp.Annotations[addonsv1alpha1.IsReleaseNameGeneratedAnnotation]
				g.Expect(ok).To(BeFalse())

				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmInstallOrUpgradeFailedReason))
				g.Expect(releaseReady.Message).To(Equal(errInternal.Error()))
			},
			expectedError: errInternal.Error(),
		},
		{
			name:             "Helm release in a failed state, no client error",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, restConfig, "", "", defaultProxy.Spec).Return(&helmRelease.Release{
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
				g.Expect(releaseReady.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmInstallOrUpgradeFailedReason))
				g.Expect(releaseReady.Message).To(Equal(fmt.Sprintf("Helm release is in a failed state: %s", helmRelease.StatusFailed)))
			},
			expectedError: "",
		},
		{
			name:             "do nothing if already successfully and installed strategy is InstallOnce",
			helmReleaseProxy: installOnceProxyAlreadyInstalled.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				// no client calls expected
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
			name:             "successfully install a Helm release when strategy is InstallOnce",
			helmReleaseProxy: installOnceProxyNotInstalled.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, restConfig, "", "", installOnceProxyNotInstalled.DeepCopy().Spec).Return(&helmRelease.Release{
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

				_, ok = hrp.Annotations[addonsv1alpha1.ReleaseSuccessfullyInstalledAnnotation]
				g.Expect(ok).To(BeTrue())

				g.Expect(hrp.Spec.ReleaseName).To(Equal("test-release"))
				g.Expect(hrp.Status.Revision).To(Equal(1))
				g.Expect(hrp.Status.Status).To(BeEquivalentTo(helmRelease.StatusDeployed))

				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
			},
			expectedError: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			clientMock := mocks.NewMockClient(mockCtrl)
			tc.clientExpect(g, clientMock.EXPECT())

			r := &HelmReleaseProxyReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(fakeScheme).
					WithStatusSubresource(&addonsv1alpha1.HelmReleaseProxy{}).
					Build(),
			}

			err := r.reconcileNormal(ctx, tc.helmReleaseProxy, clientMock, "", "", restConfig)
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

func TestReconcileNormalWithCredentialRef(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name             string
		helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
		clientExpect     func(g *WithT, c *mocks.MockClientMockRecorder)
		expect           func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy)
		expectedError    string
	}{
		{
			name:             "successfully install a Helm release",
			helmReleaseProxy: defaultProxyWithCredentialRef.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, restConfig, "/tmp/oci-credentials-xyz.json", "", defaultProxyWithCredentialRef.Spec).Return(&helmRelease.Release{
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
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			clientMock := mocks.NewMockClient(mockCtrl)
			tc.clientExpect(g, clientMock.EXPECT())

			r := &HelmReleaseProxyReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(fakeScheme).
					WithStatusSubresource(&addonsv1alpha1.HelmReleaseProxy{}).
					Build(),
			}

			err := r.reconcileNormal(ctx, tc.helmReleaseProxy, clientMock, "/tmp/oci-credentials-xyz.json", "", restConfig)
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

func TestReconcileNormalWithACertificateRef(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name             string
		helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
		clientExpect     func(g *WithT, c *mocks.MockClientMockRecorder)
		expect           func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy)
		expectedError    string
	}{
		{
			name:             "successfully install a Helm release",
			helmReleaseProxy: defaultProxyWithCACertRef.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, restConfig, "", "/tmp/ca-xyz.crt", defaultProxyWithCACertRef.Spec).Return(&helmRelease.Release{
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
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			clientMock := mocks.NewMockClient(mockCtrl)
			tc.clientExpect(g, clientMock.EXPECT())

			r := &HelmReleaseProxyReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(fakeScheme).
					WithStatusSubresource(&addonsv1alpha1.HelmReleaseProxy{}).
					Build(),
			}

			err := r.reconcileNormal(ctx, tc.helmReleaseProxy, clientMock, "", "/tmp/ca-xyz.crt", restConfig)
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
	t.Parallel()

	testcases := []struct {
		name             string
		helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
		clientExpect     func(g *WithT, c *mocks.MockClientMockRecorder)
		expect           func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy)
		expectedError    string
	}{
		{
			name:             "successfully uninstall a Helm release",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.GetHelmRelease(ctx, restConfig, defaultProxy.DeepCopy().Spec).Return(&helmRelease.Release{
					Name:    "test-release",
					Version: 1,
					Info: &helmRelease.Info{
						Status: helmRelease.StatusDeployed,
					},
				}, nil).Times(1)
				c.UninstallHelmRelease(ctx, restConfig, defaultProxy.DeepCopy().Spec).Return(&helmRelease.UninstallReleaseResponse{}, nil).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmReleaseDeletedReason))
			},
			expectedError: "",
		},
		{
			name:             "Helm release already uninstalled",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.GetHelmRelease(ctx, restConfig, defaultProxy.DeepCopy().Spec).Return(nil, helmDriver.ErrReleaseNotFound).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmReleaseDeletedReason))
			},
			expectedError: "",
		},
		{
			name:             "error attempting to get Helm release",
			helmReleaseProxy: defaultProxy.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.GetHelmRelease(ctx, restConfig, defaultProxy.DeepCopy().Spec).Return(nil, errInternal).Times(1)
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				releaseReady := conditions.Get(hrp, addonsv1alpha1.HelmReleaseReadyCondition)
				g.Expect(releaseReady.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(releaseReady.Reason).To(Equal(addonsv1alpha1.HelmReleaseGetFailedReason))
			},
			expectedError: errInternal.Error(),
		},
		{
			name:             "do nothing when strategy is InstallOnce",
			helmReleaseProxy: installOnceProxyAlreadyInstalled.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				// no client calls expected
			},
			expect: func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy) {
				// Since the condition was set to true before, it should remain true.
				g.Expect(conditions.Has(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
				g.Expect(conditions.IsTrue(hrp, addonsv1alpha1.HelmReleaseReadyCondition)).To(BeTrue())
			},
			expectedError: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			clientMock := mocks.NewMockClient(mockCtrl)
			tc.clientExpect(g, clientMock.EXPECT())

			r := &HelmReleaseProxyReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(fakeScheme).
					WithStatusSubresource(&addonsv1alpha1.HelmReleaseProxy{}).
					Build(),
			}

			err := r.reconcileDelete(ctx, tc.helmReleaseProxy, clientMock, restConfig)
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

func TestTLSSettings(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name             string
		helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
		clientExpect     func(g *WithT, c *mocks.MockClientMockRecorder)
		expect           func(g *WithT, hrp *addonsv1alpha1.HelmReleaseProxy)
		expectedError    string
	}{
		{
			name:             "test",
			helmReleaseProxy: defaultProxyWithSkipTLS.DeepCopy(),
			clientExpect: func(g *WithT, c *mocks.MockClientMockRecorder) {
				c.InstallOrUpgradeHelmRelease(ctx, restConfig, "", "", defaultProxyWithSkipTLS.Spec).Return(&helmRelease.Release{
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
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			clientMock := mocks.NewMockClient(mockCtrl)
			tc.clientExpect(g, clientMock.EXPECT())

			r := &HelmReleaseProxyReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(fakeScheme).
					WithStatusSubresource(&addonsv1alpha1.HelmReleaseProxy{}).
					Build(),
			}
			caFilePath, err := r.getCAFile(ctx, tc.helmReleaseProxy)
			g.Expect(err).ToNot(HaveOccurred(), "did not expect error to get CA file")
			err = r.reconcileNormal(ctx, tc.helmReleaseProxy, clientMock, "", caFilePath, restConfig)
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

func init() {
	_ = scheme.AddToScheme(fakeScheme)
	_ = clusterv1.AddToScheme(fakeScheme)
	_ = addonsv1alpha1.AddToScheme(fakeScheme)
}
