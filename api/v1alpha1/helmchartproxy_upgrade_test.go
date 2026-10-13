/*
Copyright The Kubernetes Authors.

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const legacySecretReferenceNamespace = "helmchartproxy-upgrade"

var legacySecretReferenceCases = []struct {
	name        string
	credentials bool
	ca          bool
}{
	{name: "credentials", credentials: true},
	{name: "ca", ca: true},
	{name: "both", credentials: true, ca: true},
}

func createLegacyHelmChartProxies() {
	GinkgoHelper()

	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: legacySecretReferenceNamespace},
	})).To(Succeed())

	for _, tc := range legacySecretReferenceCases {
		for _, operation := range []string{"metadata", "finalizer"} {
			proxy := &HelmChartProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:       tc.name + "-" + operation,
					Namespace:  legacySecretReferenceNamespace,
					Finalizers: []string{HelmChartProxyFinalizer},
				},
				Spec: HelmChartProxySpec{
					ChartName:        "example",
					RepoURL:          "https://charts.example.com",
					ReleaseNamespace: metav1.NamespaceDefault,
					Options: HelmOptions{
						Timeout: &metav1.Duration{Duration: helmTimeout},
					},
				},
			}
			if tc.credentials {
				proxy.Spec.Credentials = &Credentials{
					Secret: corev1.SecretReference{Name: "registry", Namespace: "caaph-system"},
					Key:    DefaultOCIKey,
				}
			}
			if tc.ca {
				proxy.Spec.TLSConfig = &TLSConfig{
					CASecretRef: &corev1.SecretReference{Name: "ca", Namespace: "caaph-system"},
				}
			}

			Expect(k8sClient.Create(ctx, proxy)).To(Succeed())
		}
	}
}

var _ = Describe("HelmChartProxy legacy Secret references", func() {
	for _, tc := range legacySecretReferenceCases {
		Context(tc.name, func() {
			It("allows metadata-only updates after admission is upgraded", func() {
				key := client.ObjectKey{
					Namespace: legacySecretReferenceNamespace,
					Name:      tc.name + "-metadata",
				}
				proxy := &HelmChartProxy{}
				Expect(k8sClient.Get(ctx, key, proxy)).To(Succeed())

				patch := client.MergeFrom(proxy.DeepCopy())
				proxy.Annotations = map[string]string{"cluster.x-k8s.io/paused": ""}
				Expect(k8sClient.Patch(ctx, proxy, patch)).To(Succeed())

				Expect(k8sClient.Get(ctx, key, proxy)).To(Succeed())
				Expect(proxy.Annotations).To(HaveKey("cluster.x-k8s.io/paused"))
			})

			It("allows finalizer removal after admission is upgraded", func() {
				key := client.ObjectKey{
					Namespace: legacySecretReferenceNamespace,
					Name:      tc.name + "-finalizer",
				}
				proxy := &HelmChartProxy{}
				Expect(k8sClient.Get(ctx, key, proxy)).To(Succeed())

				Expect(k8sClient.Delete(ctx, proxy)).To(Succeed())
				Expect(k8sClient.Get(ctx, key, proxy)).To(Succeed())
				Expect(proxy.DeletionTimestamp.IsZero()).To(BeFalse())
				Expect(controllerutil.ContainsFinalizer(proxy, HelmChartProxyFinalizer)).To(BeTrue())

				patch := client.MergeFrom(proxy.DeepCopy())
				controllerutil.RemoveFinalizer(proxy, HelmChartProxyFinalizer)
				Expect(k8sClient.Patch(ctx, proxy, patch)).To(Succeed())

				err := k8sClient.Get(ctx, key, &HelmChartProxy{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected deletion to complete, got %v", err)
			})
		})
	}
})
