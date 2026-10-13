/*
Copyright 2026 The Kubernetes Authors.

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
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretReferenceValidation(t *testing.T) {
	t.Parallel()

	newProxy := func(credentialsNamespace, caNamespace string) *HelmChartProxy {
		proxy := &HelmChartProxy{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "tenant-a"},
		}
		if credentialsNamespace != "-" {
			proxy.Spec.Credentials = &Credentials{
				Secret: corev1.SecretReference{Name: "credentials", Namespace: credentialsNamespace},
				Key:    "config.json",
			}
		}
		if caNamespace != "-" {
			proxy.Spec.TLSConfig = &TLSConfig{
				CASecretRef: &corev1.SecretReference{Name: "ca", Namespace: caNamespace},
			}
		}

		return proxy
	}

	testCases := []struct {
		name      string
		proxy     *HelmChartProxy
		expectErr bool
	}{
		{name: "no secret references", proxy: newProxy("-", "-")},
		{name: "empty credentials namespace", proxy: newProxy("", "-")},
		{name: "matching credentials namespace", proxy: newProxy("tenant-a", "-")},
		{name: "foreign credentials namespace", proxy: newProxy("kube-system", "-"), expectErr: true},
		{name: "empty CA namespace", proxy: newProxy("-", "")},
		{name: "matching CA namespace", proxy: newProxy("-", "tenant-a")},
		{name: "foreign CA namespace", proxy: newProxy("-", "kube-system"), expectErr: true},
		{name: "both foreign", proxy: newProxy("kube-system", "kube-system"), expectErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			allErrs := validateSecretReferences(tc.proxy)
			if tc.expectErr {
				g.Expect(allErrs).NotTo(BeEmpty())
			} else {
				g.Expect(allErrs).To(BeEmpty())
			}
		})
	}
}

func TestWebhookRejectsForeignSecretReferences(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	proxy := &HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "tenant-a"},
		Spec: HelmChartProxySpec{
			RepoURL: "https://charts.example.com",
			Credentials: &Credentials{
				Secret: corev1.SecretReference{Name: "credentials", Namespace: "tenant-b"},
			},
		},
	}
	webhook := &helmChartProxyWebhook{}

	_, err := webhook.ValidateCreate(context.Background(), proxy)
	g.Expect(err).To(HaveOccurred())

	oldProxy := proxy.DeepCopy()
	oldProxy.Spec.Credentials.Secret.Namespace = oldProxy.Namespace
	_, err = webhook.ValidateUpdate(context.Background(), oldProxy, proxy)
	g.Expect(err).To(HaveOccurred())
}
