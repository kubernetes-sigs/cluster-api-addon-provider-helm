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
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// releaseDriftReconciler reconciles an event from the all helm objects managed by the HelmReleaseProxy.
type releaseDriftReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	HelmReleaseProxyKey   client.ObjectKey
	HelmReleaseProxyEvent chan event.GenericEvent
}

var excludeCreateEventsPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		return shouldFilteredByManager(e.ObjectNew.GetManagedFields())

	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return shouldFilteredByManager(e.Object.GetManagedFields())
	},
}

func shouldFilteredByManager(mfs []metav1.ManagedFieldsEntry) bool {
	mfl := len(mfs)
	if mfl > 0 {
		manager := mfs[mfl-1].Manager
		return !(manager == os.Args[0])
	}

	return false
}

// setupWithManager sets up the controller with the Manager.
func (r *releaseDriftReconciler) setupWithManager(mgr ctrl.Manager, gvks []schema.GroupVersionKind) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		Named(fmt.Sprintf("%s-%s-release-drift-controller", r.HelmReleaseProxyKey.Name, r.HelmReleaseProxyKey.Namespace))
	for _, gvk := range gvks {
		watch := &unstructured.Unstructured{}
		watch.SetGroupVersionKind(gvk)
		controllerBuilder.Watches(watch, handler.EnqueueRequestsFromMapFunc(r.WatchesToReleaseMapper), builder.OnlyMetadata)
	}

	return controllerBuilder.WithEventFilter(excludeCreateEventsPredicate).Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *releaseDriftReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(2).Info("Beginning reconciliation", "requestNamespace", req.Namespace, "requestName", req.Name)

	objectMeta := metav1.ObjectMeta{
		Name:      r.HelmReleaseProxyKey.Name,
		Namespace: r.HelmReleaseProxyKey.Namespace,
	}
	r.HelmReleaseProxyEvent <- event.GenericEvent{Object: &addonsv1alpha1.HelmReleaseProxy{ObjectMeta: objectMeta}}

	return ctrl.Result{}, nil
}

func (r *releaseDriftReconciler) WatchesToReleaseMapper(_ context.Context, _ client.Object) []ctrl.Request {
	return []ctrl.Request{{NamespacedName: r.HelmReleaseProxyKey}}
}
