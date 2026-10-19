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
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretReferenceValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		proxy         *HelmChartProxy
		invalidFields []string
	}{
		{name: "no secret references", proxy: newSecretReferenceTestProxy("-", "-")},
		{name: "empty credentials namespace", proxy: newSecretReferenceTestProxy("", "-")},
		{name: "matching credentials namespace", proxy: newSecretReferenceTestProxy("tenant-a", "-")},
		{
			name:          "foreign credentials namespace",
			proxy:         newSecretReferenceTestProxy("kube-system", "-"),
			invalidFields: []string{"spec.credentials.secret.namespace"},
		},
		{name: "empty CA namespace", proxy: newSecretReferenceTestProxy("-", "")},
		{name: "matching CA namespace", proxy: newSecretReferenceTestProxy("-", "tenant-a")},
		{
			name:          "foreign CA namespace",
			proxy:         newSecretReferenceTestProxy("-", "kube-system"),
			invalidFields: []string{"spec.tlsConfig.caSecret.namespace"},
		},
		{
			name:          "both foreign",
			proxy:         newSecretReferenceTestProxy("kube-system", "kube-system"),
			invalidFields: []string{"spec.credentials.secret.namespace", "spec.tlsConfig.caSecret.namespace"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			allErrs := validateSecretReferences(nil, tc.proxy)
			g.Expect(allErrs).To(HaveLen(len(tc.invalidFields)))
			for i, field := range tc.invalidFields {
				g.Expect(allErrs[i].Field).To(Equal(field))
			}

			_, err := (&helmChartProxyWebhook{}).ValidateCreate(context.Background(), tc.proxy)
			if len(tc.invalidFields) > 0 {
				g.Expect(apierrors.IsInvalid(err)).To(BeTrue())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestWebhookAllowsUnchangedLegacySecretReferences(t *testing.T) {
	t.Parallel()

	references := []struct {
		name  string
		proxy *HelmChartProxy
	}{
		{name: "credentials", proxy: newSecretReferenceTestProxy("kube-system", "-")},
		{name: "CA", proxy: newSecretReferenceTestProxy("-", "kube-system")},
		{name: "both", proxy: newSecretReferenceTestProxy("kube-system", "kube-system")},
	}
	updates := []struct {
		name   string
		update func(*HelmChartProxy)
	}{
		{
			name: "metadata",
			update: func(proxy *HelmChartProxy) {
				proxy.Annotations = map[string]string{"cluster.x-k8s.io/paused": ""}
			},
		},
		{
			name: "finalizer removal",
			update: func(proxy *HelmChartProxy) {
				proxy.Finalizers = nil
			},
		},
		{
			name: "unrelated spec",
			update: func(proxy *HelmChartProxy) {
				proxy.Spec.Version = "1.2.3"
			},
		},
		{
			name: "unrelated TLS setting",
			update: func(proxy *HelmChartProxy) {
				if proxy.Spec.TLSConfig == nil {
					proxy.Spec.TLSConfig = &TLSConfig{}
				}
				proxy.Spec.TLSConfig.InsecureSkipTLSVerify = true
			},
		},
	}

	for _, reference := range references {
		for _, update := range updates {
			t.Run(reference.name+"/"+update.name, func(t *testing.T) {
				t.Parallel()
				g := NewWithT(t)

				oldProxy := reference.proxy.DeepCopy()
				oldProxy.Finalizers = []string{HelmChartProxyFinalizer}
				proxy := oldProxy.DeepCopy()
				update.update(proxy)

				_, err := (&helmChartProxyWebhook{}).ValidateUpdate(context.Background(), oldProxy, proxy)
				g.Expect(err).NotTo(HaveOccurred())
			})
		}
	}
}

func TestWebhookSecretReferenceUpdates(t *testing.T) {
	t.Parallel()

	renamedCredentials := newSecretReferenceTestProxy("kube-system", "kube-system")
	renamedCredentials.Spec.Credentials.Secret.Name = "other-credentials"
	renamedCA := newSecretReferenceTestProxy("kube-system", "kube-system")
	renamedCA.Spec.TLSConfig.CASecretRef.Name = "other-ca"
	removedCA := newSecretReferenceTestProxy("kube-system", "-")
	removedCA.Spec.TLSConfig = &TLSConfig{}
	invalidURL := newSecretReferenceTestProxy("kube-system", "kube-system")
	invalidURL.Spec.RepoURL = "not-a-url"
	changedStrategy := newSecretReferenceTestProxy("kube-system", "kube-system")
	changedStrategy.Spec.ReconcileStrategy = string(ReconcileStrategyInstallOnce)

	testCases := []struct {
		name          string
		oldProxy      *HelmChartProxy
		proxy         *HelmChartProxy
		invalidFields []string
	}{
		{
			name:     "no references",
			oldProxy: newSecretReferenceTestProxy("-", "-"),
			proxy:    newSecretReferenceTestProxy("-", "-"),
		},
		{
			name:     "introducing implicit local references",
			oldProxy: newSecretReferenceTestProxy("-", "-"),
			proxy:    newSecretReferenceTestProxy("", ""),
		},
		{
			name:     "introducing explicit local references",
			oldProxy: newSecretReferenceTestProxy("-", "-"),
			proxy:    newSecretReferenceTestProxy("tenant-a", "tenant-a"),
		},
		{
			name:          "introducing foreign credentials",
			oldProxy:      newSecretReferenceTestProxy("-", "-"),
			proxy:         newSecretReferenceTestProxy("kube-system", "-"),
			invalidFields: []string{"spec.credentials.secret.namespace"},
		},
		{
			name:          "introducing foreign CA",
			oldProxy:      newSecretReferenceTestProxy("-", "-"),
			proxy:         newSecretReferenceTestProxy("-", "kube-system"),
			invalidFields: []string{"spec.tlsConfig.caSecret.namespace"},
		},
		{
			name:          "introducing both foreign references",
			oldProxy:      newSecretReferenceTestProxy("-", "-"),
			proxy:         newSecretReferenceTestProxy("kube-system", "kube-system"),
			invalidFields: []string{"spec.credentials.secret.namespace", "spec.tlsConfig.caSecret.namespace"},
		},
		{
			name:          "changing implicit local credentials to foreign",
			oldProxy:      newSecretReferenceTestProxy("", "-"),
			proxy:         newSecretReferenceTestProxy("kube-system", "-"),
			invalidFields: []string{"spec.credentials.secret.namespace"},
		},
		{
			name:          "changing explicit local credentials to foreign",
			oldProxy:      newSecretReferenceTestProxy("tenant-a", "-"),
			proxy:         newSecretReferenceTestProxy("kube-system", "-"),
			invalidFields: []string{"spec.credentials.secret.namespace"},
		},
		{
			name:          "changing implicit local CA to foreign",
			oldProxy:      newSecretReferenceTestProxy("-", ""),
			proxy:         newSecretReferenceTestProxy("-", "kube-system"),
			invalidFields: []string{"spec.tlsConfig.caSecret.namespace"},
		},
		{
			name:          "changing explicit local CA to foreign",
			oldProxy:      newSecretReferenceTestProxy("-", "tenant-a"),
			proxy:         newSecretReferenceTestProxy("-", "kube-system"),
			invalidFields: []string{"spec.tlsConfig.caSecret.namespace"},
		},
		{
			name:          "changing foreign credentials namespace",
			oldProxy:      newSecretReferenceTestProxy("tenant-b", "-"),
			proxy:         newSecretReferenceTestProxy("kube-system", "-"),
			invalidFields: []string{"spec.credentials.secret.namespace"},
		},
		{
			name:          "changing foreign CA namespace",
			oldProxy:      newSecretReferenceTestProxy("-", "tenant-b"),
			proxy:         newSecretReferenceTestProxy("-", "kube-system"),
			invalidFields: []string{"spec.tlsConfig.caSecret.namespace"},
		},
		{
			name:          "changing foreign credentials name with unchanged legacy CA",
			oldProxy:      newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:         renamedCredentials,
			invalidFields: []string{"spec.credentials.secret.namespace"},
		},
		{
			name:          "changing foreign CA name with unchanged legacy credentials",
			oldProxy:      newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:         renamedCA,
			invalidFields: []string{"spec.tlsConfig.caSecret.namespace"},
		},
		{
			name:          "introducing foreign CA with unchanged legacy credentials",
			oldProxy:      newSecretReferenceTestProxy("kube-system", "-"),
			proxy:         newSecretReferenceTestProxy("kube-system", "kube-system"),
			invalidFields: []string{"spec.tlsConfig.caSecret.namespace"},
		},
		{
			name:          "introducing foreign CA into existing TLS config",
			oldProxy:      removedCA,
			proxy:         newSecretReferenceTestProxy("kube-system", "kube-system"),
			invalidFields: []string{"spec.tlsConfig.caSecret.namespace"},
		},
		{
			name:          "introducing foreign credentials with unchanged legacy CA",
			oldProxy:      newSecretReferenceTestProxy("-", "kube-system"),
			proxy:         newSecretReferenceTestProxy("kube-system", "kube-system"),
			invalidFields: []string{"spec.credentials.secret.namespace"},
		},
		{
			name:     "correcting both namespaces to implicit local references",
			oldProxy: newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:    newSecretReferenceTestProxy("", ""),
		},
		{
			name:     "correcting both namespaces to explicit local references",
			oldProxy: newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:    newSecretReferenceTestProxy("tenant-a", "tenant-a"),
		},
		{
			name:     "correcting credentials with unchanged legacy CA",
			oldProxy: newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:    newSecretReferenceTestProxy("tenant-a", "kube-system"),
		},
		{
			name:     "correcting CA with unchanged legacy credentials",
			oldProxy: newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:    newSecretReferenceTestProxy("kube-system", "tenant-a"),
		},
		{
			name:     "removing both legacy references",
			oldProxy: newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:    newSecretReferenceTestProxy("-", "-"),
		},
		{
			name:     "removing CA reference without removing TLS config",
			oldProxy: newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:    removedCA,
		},
		{
			name:          "invalid URL with unchanged legacy references",
			oldProxy:      newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:         invalidURL,
			invalidFields: []string{"spec.RepoURL"},
		},
		{
			name:          "immutable strategy with unchanged legacy references",
			oldProxy:      newSecretReferenceTestProxy("kube-system", "kube-system"),
			proxy:         changedStrategy,
			invalidFields: []string{"spec.ReconcileStrategy"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			_, err := (&helmChartProxyWebhook{}).ValidateUpdate(context.Background(), tc.oldProxy, tc.proxy)
			if len(tc.invalidFields) == 0 {
				g.Expect(err).NotTo(HaveOccurred())
				return
			}

			g.Expect(apierrors.IsInvalid(err)).To(BeTrue())
			var statusErr *apierrors.StatusError
			g.Expect(errors.As(err, &statusErr)).To(BeTrue())
			g.Expect(statusErr.ErrStatus.Details.Causes).To(HaveLen(len(tc.invalidFields)))
			for i, field := range tc.invalidFields {
				g.Expect(statusErr.ErrStatus.Details.Causes[i].Field).To(Equal(field))
			}
		})
	}
}

func newSecretReferenceTestProxy(credentialsNamespace, caNamespace string) *HelmChartProxy {
	proxy := &HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "tenant-a"},
		Spec:       HelmChartProxySpec{RepoURL: "https://charts.example.com"},
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
