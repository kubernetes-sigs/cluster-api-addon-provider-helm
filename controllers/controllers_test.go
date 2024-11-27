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

package controllers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	helmrelease "helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testNamespace1       = "test-namespace1"
	testNamespace2       = "test-namespace2"
	newVersion           = "new-version"
	releaseFailedMessage = "unable to remove helm release"
)

var (
	namespaces          = []string{testNamespace1, testNamespace2}
	failedHelmUninstall bool

	newProxy = func(namespace string) *addonsv1alpha1.HelmChartProxy {
		return &addonsv1alpha1.HelmChartProxy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: addonsv1alpha1.GroupVersion.String(),
				Kind:       "HelmChartProxy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-hcp",
				Namespace: namespace,
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
			},
		}
	}

	newCluster = func(namespace string) *clusterv1.Cluster {
		return &clusterv1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-1",
				Namespace: namespace,
				Labels: map[string]string{
					"test-label": "test-value",
				},
			},
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					APIServerPort: ptr.To(int32(1234)),
				},
			},
		}
	}

	helmReleaseDeployed = &helmrelease.Release{
		Name:    "test-release",
		Version: 1,
		Info: &helmrelease.Info{
			Status: helmrelease.StatusDeployed,
		},
	}
)

func newKubeconfigSecretForCluster(cluster *clusterv1.Cluster) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-kubeconfig",
			Namespace: cluster.Namespace,
		},
		StringData: map[string]string{
			secret.KubeconfigDataName: `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://127.0.0.1:8080
  name: ` + cluster.Name + `
contexts:
- context:
    cluster: ` + cluster.Name + `
  name: ` + cluster.Name + `
current-context: ` + cluster.Name + `
`,
		},
	}
}

var _ = Describe("Testing HelmChartProxy and HelmReleaseProxy reconcile", func() {
	var (
		waitForHelmChartProxyCondition = func(objectKey client.ObjectKey, condition func(helmChartProxy *addonsv1alpha1.HelmChartProxy) bool) {
			hcp := &addonsv1alpha1.HelmChartProxy{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, objectKey, hcp); err != nil {
					return false
				}

				return condition != nil && condition(hcp)
			}, timeout, interval).Should(BeTrue())
		}

		waitForHelmReleaseProxyCondition = func(helmChartProxyKey client.ObjectKey, condition func(helmReleaseProxyList []addonsv1alpha1.HelmReleaseProxy) bool) {
			hrpList := &addonsv1alpha1.HelmReleaseProxyList{}
			Eventually(func() bool {
				if err := k8sClient.List(ctx, hrpList, client.InNamespace(helmChartProxyKey.Namespace), client.MatchingLabels(map[string]string{addonsv1alpha1.HelmChartProxyLabelName: helmChartProxyKey.Name})); err != nil {
					return false
				}

				return condition != nil && condition(hrpList.Items)
			}, timeout, interval).Should(BeTrue())
		}

		install = func(cluster *clusterv1.Cluster, proxy *addonsv1alpha1.HelmChartProxy) {
			err := k8sClient.Create(ctx, cluster)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Create(ctx, newKubeconfigSecretForCluster(cluster))
			Expect(err).ToNot(HaveOccurred())

			patch := client.MergeFrom(cluster.DeepCopy())
			conditions.MarkTrue(cluster, clusterv1.ControlPlaneInitializedCondition)
			err = k8sClient.Status().Patch(ctx, cluster, patch)
			Expect(err).ToNot(HaveOccurred())

			err = k8sClient.Create(ctx, proxy)
			Expect(err).ToNot(HaveOccurred())

			waitForHelmChartProxyCondition(client.ObjectKeyFromObject(proxy), func(helmChartProxy *addonsv1alpha1.HelmChartProxy) bool {
				return conditions.IsTrue(helmChartProxy, clusterv1.ReadyCondition)
			})

			waitForHelmReleaseProxyCondition(client.ObjectKeyFromObject(proxy), func(helmReleaseProxyList []addonsv1alpha1.HelmReleaseProxy) bool {
				return len(helmReleaseProxyList) == 1 && conditions.IsTrue(&helmReleaseProxyList[0], clusterv1.ReadyCondition)
			})
		}

		deleteAndWaitHelmChartProxy = func(proxy *addonsv1alpha1.HelmChartProxy) {
			err := k8sClient.Delete(ctx, proxy)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(proxy), &addonsv1alpha1.HelmChartProxy{}); client.IgnoreNotFound(err) != nil {
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())

			waitForHelmReleaseProxyCondition(client.ObjectKeyFromObject(proxy), func(helmReleaseProxyList []addonsv1alpha1.HelmReleaseProxy) bool {
				return len(helmReleaseProxyList) == 0
			})
		}
	)

	It("HelmChartProxy and HelmReleaseProxy lifecycle happy path test", func() {
		cluster := newCluster(testNamespace1)
		helmChartProxy := newProxy(testNamespace1)
		install(cluster, helmChartProxy)

		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(helmChartProxy), helmChartProxy)
		Expect(err).ToNot(HaveOccurred())
		patch := client.MergeFrom(helmChartProxy.DeepCopy())
		helmChartProxy.Spec.Version = newVersion
		err = k8sClient.Patch(ctx, helmChartProxy, patch)
		Expect(err).ToNot(HaveOccurred())

		waitForHelmReleaseProxyCondition(client.ObjectKeyFromObject(helmChartProxy), func(helmReleaseProxyList []addonsv1alpha1.HelmReleaseProxy) bool {
			return len(helmReleaseProxyList) == 1 && conditions.IsTrue(&helmReleaseProxyList[0], clusterv1.ReadyCondition) && helmReleaseProxyList[0].Spec.Version == "new-version"
		})

		deleteAndWaitHelmChartProxy(helmChartProxy)
	})

	It("HelmChartProxy and HelmReleaseProxy test with failed Release uninstall", func() {
		cluster := newCluster(testNamespace2)
		helmChartProxy := newProxy(testNamespace2)
		failedHelmUninstall = true
		install(cluster, helmChartProxy)

		err := k8sClient.Delete(ctx, helmChartProxy)
		Expect(err).ToNot(HaveOccurred())

		Consistently(func() bool {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(helmChartProxy), helmChartProxy); err != nil {
				return false
			}

			return true
		}, timeout, interval).Should(BeTrue())

		readyCondition := conditions.Get(helmChartProxy, clusterv1.ReadyCondition)
		Expect(readyCondition).NotTo(BeNil())
		Expect(readyCondition.Status).To(Equal(corev1.ConditionFalse))
		Expect(readyCondition.Message).To(Equal(releaseFailedMessage))

		By("Making HelmChartProxy uninstallable")
		failedHelmUninstall = false
		deleteAndWaitHelmChartProxy(helmChartProxy)
	})
})
