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

package fake

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ManifestReconciler reconciles an event from all fake manifest objects channel.
type ManifestReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ManifestObjects []client.Object
}

var ManifestEventChannel = make(chan event.GenericEvent, 1)

// SetupWithManager sets up the controller with the Manager.
func (r *ManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("fake-manifest-controller").
		WatchesRawSource(source.Channel(ManifestEventChannel, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ManifestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(2).Info("Beginning reconciliation", "requestNamespace", req.Namespace, "requestName", req.Name)

	for _, object := range r.ManifestObjects {
		object.SetNamespace(req.Namespace)
		requestObject, _ := object.DeepCopyObject().(client.Object)
		err := r.Get(ctx, client.ObjectKeyFromObject(object), requestObject)
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		if apierrors.IsNotFound(err) {
			if err = r.Client.Create(ctx, requestObject); err != nil {
				return ctrl.Result{}, err
			}

			continue
		}
		if err = r.Client.Update(ctx, object); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
