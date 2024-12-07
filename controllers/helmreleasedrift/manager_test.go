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

package helmreleasedrift_test

import (
	"github.com/ironcore-dev/controller-utils/metautils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"sigs.k8s.io/cluster-api-addon-provider-helm/controllers/helmreleasedrift"
	"sigs.k8s.io/cluster-api-addon-provider-helm/controllers/helmreleasedrift/test/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const (
	releaseName                = "ahoy"
	objectName                 = "ahoy-hello-world"
	originalDeploymentReplicas = 1
	patchedDeploymentReplicas  = 3
)

var _ = Describe("Testing HelmReleaseProxy drift manager with fake manifest", func() {
	It("Adding HelmReleaseProxy drift manager and validating its lifecycle", func() {
		objectMeta := metav1.ObjectMeta{
			Name:      releaseName,
			Namespace: metav1.NamespaceDefault,
		}
		fake.ManifestEventChannel <- event.GenericEvent{Object: &addonsv1alpha1.HelmReleaseProxy{ObjectMeta: objectMeta}}

		helmReleaseProxy := &addonsv1alpha1.HelmReleaseProxy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ahoy-release-proxy",
				Namespace: metav1.NamespaceDefault,
			},
			Spec: addonsv1alpha1.HelmReleaseProxySpec{
				ReleaseName: releaseName,
			},
		}

		// TODO (dvolodin) Find way how to wait manager to start for testing
		err := helmreleasedrift.Add(ctx, restConfig, helmReleaseProxy, manifest, fake.ManifestEventChannel)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			for _, objectList := range []client.ObjectList{&corev1.ServiceList{}, &appsv1.DeploymentList{}, &corev1.ServiceAccountList{}} {
				err := k8sClient.List(ctx, objectList, client.InNamespace(metav1.NamespaceDefault), client.MatchingLabels(map[string]string{helmreleasedrift.InstanceLabelKey: releaseName}))
				if err != nil {
					return false
				}
				objects, err := metautils.ExtractList(objectList)
				if err != nil || len(objects) == 0 {
					return false
				}
			}

			return true
		}, timeout, interval).Should(BeTrue())

		deployment := &appsv1.Deployment{}
		err = k8sClient.Get(ctx, client.ObjectKey{Name: objectName, Namespace: metav1.NamespaceDefault}, deployment)
		Expect(err).NotTo(HaveOccurred())
		patch := client.MergeFrom(deployment.DeepCopy())
		deployment.Spec.Replicas = ptr.To(int32(patchedDeploymentReplicas))
		err = k8sClient.Patch(ctx, deployment, patch)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			err = k8sClient.Get(ctx, client.ObjectKey{Name: objectName, Namespace: metav1.NamespaceDefault}, deployment)
			return err == nil && *deployment.Spec.Replicas == originalDeploymentReplicas
		}, timeout, interval).Should(BeTrue())

		helmreleasedrift.Remove(helmReleaseProxy)
	})
})
