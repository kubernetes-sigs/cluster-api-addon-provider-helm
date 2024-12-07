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

package helmreleasedrift

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ironcore-dev/controller-utils/unstructuredutils"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/kustomize/api/konfig"
)

const (
	InstanceLabelKey = "app.kubernetes.io/instance"
)

var (
	managers = map[string]options{}
	mutex    sync.Mutex
)

type options struct {
	gvks   []schema.GroupVersionKind
	cancel context.CancelFunc
}

func Add(ctx context.Context, restConfig *rest.Config, helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy, releaseManifest string, eventChannel chan event.GenericEvent) error {
	log := ctrl.LoggerFrom(ctx)
	gvks, err := extractGVKsFromManifest(releaseManifest)
	if err != nil {
		return err
	}

	manager, exist := managers[managerKey(helmReleaseProxy)]
	if exist {
		if slices.Equal(manager.gvks, gvks) {
			return nil
		}
		Remove(helmReleaseProxy)
	}

	mutex.Lock()
	defer mutex.Unlock()
	k8sManager, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
		Cache: cache.Options{
			DefaultLabelSelector: labels.SelectorFromSet(map[string]string{
				konfig.ManagedbyLabelKey: "Helm",
				InstanceLabelKey:         helmReleaseProxy.Spec.ReleaseName,
			}),
		},
	})
	if err != nil {
		return err
	}
	if err = (&releaseDriftReconciler{
		Client:                k8sManager.GetClient(),
		Scheme:                k8sManager.GetScheme(),
		HelmReleaseProxyKey:   client.ObjectKeyFromObject(helmReleaseProxy),
		HelmReleaseProxyEvent: eventChannel,
	}).setupWithManager(k8sManager, gvks); err != nil {
		return err
	}
	log.V(2).Info("Starting release drift controller manager")
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		if err = k8sManager.Start(ctx); err != nil {
			log.V(2).Error(err, "failed to start release drift manager")
			objectMeta := metav1.ObjectMeta{
				Name:      helmReleaseProxy.Name,
				Namespace: helmReleaseProxy.Namespace,
			}
			eventChannel <- event.GenericEvent{Object: &addonsv1alpha1.HelmReleaseProxy{ObjectMeta: objectMeta}}
		}
	}()

	managers[managerKey(helmReleaseProxy)] = options{
		gvks:   gvks,
		cancel: cancel,
	}

	return nil
}

func Remove(helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy) {
	mutex.Lock()
	defer mutex.Unlock()

	manager, exist := managers[managerKey(helmReleaseProxy)]
	if exist {
		manager.cancel()
		delete(managers, managerKey(helmReleaseProxy))
	}
}

func managerKey(helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy) string {
	return fmt.Sprintf("%s-%s-%s", helmReleaseProxy.Spec.ClusterRef.Name, helmReleaseProxy.Namespace, helmReleaseProxy.Spec.ReleaseName)
}

func extractGVKsFromManifest(manifest string) ([]schema.GroupVersionKind, error) {
	objects, err := unstructuredutils.Read(strings.NewReader(manifest))
	if err != nil {
		return nil, err
	}
	var gvks []schema.GroupVersionKind
	for _, obj := range objects {
		if !slices.Contains(gvks, obj.GroupVersionKind()) {
			gvks = append(gvks, obj.GroupVersionKind())
		}
	}

	return gvks, nil
}
