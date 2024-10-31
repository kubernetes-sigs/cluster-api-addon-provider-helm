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

package helmchartproxy

import (
	"context"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// HelmChartProxyReconciler reconciles a HelmChartProxy object.
type HelmChartProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmChartProxyReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&addonsv1alpha1.HelmChartProxy{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToHelmChartProxiesMapper),
		).
		Watches(
			&addonsv1alpha1.HelmReleaseProxy{},
			handler.EnqueueRequestsFromMapFunc(HelmReleaseProxyToHelmChartProxyMapper),
		).
		Complete(r)
}

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/finalizers,verbs=update
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=list;watch
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=list;get;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io;clusterctl.cluster.x-k8s.io,resources=*,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=list;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HelmChartProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Beginning reconciliation for HelmChartProxy", "requestNamespace", req.Namespace, "requestName", req.Name)

	// Fetch the HelmChartProxy instance.
	helmChartProxy := &addonsv1alpha1.HelmChartProxy{}
	if err := r.Client.Get(ctx, req.NamespacedName, helmChartProxy); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("HelmChartProxy resource not found, skipping reconciliation", "helmChartProxy", req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// TODO: should patch helper return an error when the object has been deleted?
	patchHelper, err := patch.NewHelper(helmChartProxy, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to init patch helper")
	}

	defer func() {
		log.V(2).Info("Preparing to patch HelmChartProxy", "helmChartProxy", helmChartProxy.Name)
		if err := patchHelmChartProxy(ctx, patchHelper, helmChartProxy); err != nil && reterr == nil {
			reterr = err
			log.Error(err, "failed to patch HelmChartProxy", "helmChartProxy", helmChartProxy.Name)

			return
		}
		log.V(2).Info("Successfully patched HelmChartProxy", "helmChartProxy", helmChartProxy.Name)
	}()

	selector := helmChartProxy.Spec.ClusterSelector

	log.V(2).Info("Finding matching clusters for HelmChartProxy with selector selector", "helmChartProxy", helmChartProxy.Name, "selector", selector)
	// TODO: When a Cluster is being deleted, it will show up in the list of clusters even though we can't Reconcile on it.
	// This is because of ownerRefs and how the Cluster gets deleted. It will be eventually consistent but it would be better
	// to not have errors. An idea would be to check the deletion timestamp.
	clusterList, err := r.listClustersWithLabels(ctx, helmChartProxy.Namespace, selector)
	if err != nil {
		conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition, addonsv1alpha1.ClusterSelectionFailedReason, clusterv1.ConditionSeverityError, "%s", err.Error())

		return ctrl.Result{}, err
	}
	// conditions.MarkTrue(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsReadyCondition)
	helmChartProxy.SetMatchingClusters(clusterList.Items)

	log.V(2).Info("Finding HelmRelease for HelmChartProxy", "helmChartProxy", helmChartProxy.Name)
	label := map[string]string{
		addonsv1alpha1.HelmChartProxyLabelName: helmChartProxy.Name,
	}
	releaseList, err := r.listInstalledReleases(ctx, helmChartProxy.Namespace, label)
	if err != nil {
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if helmChartProxy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(helmChartProxy, addonsv1alpha1.HelmChartProxyFinalizer) {
			controllerutil.AddFinalizer(helmChartProxy, addonsv1alpha1.HelmChartProxyFinalizer)
			if err := patchHelmChartProxy(ctx, patchHelper, helmChartProxy); err != nil {
				// TODO: Should we try to set the error here? If we can't add the finalizer we likely can't update the status either.
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(helmChartProxy, addonsv1alpha1.HelmChartProxyFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.reconcileDelete(ctx, helmChartProxy, releaseList.Items); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(helmChartProxy, addonsv1alpha1.HelmChartProxyFinalizer)
			if err := patchHelmChartProxy(ctx, patchHelper, helmChartProxy); err != nil {
				// TODO: Should we try to set the error here? If we can't remove the finalizer we likely can't update the status either.
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	log.V(2).Info("Reconciling HelmChartProxy", "randomName", helmChartProxy.Name)
	err = r.reconcileNormal(ctx, helmChartProxy, clusterList.Items, releaseList.Items)
	if err != nil {
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition)

	err = r.aggregateHelmReleaseProxyReadyCondition(ctx, helmChartProxy)
	if err != nil {
		log.Error(err, "failed to aggregate HelmReleaseProxy ready condition", "helmChartProxy", helmChartProxy.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileNormal handles the reconciliation of a HelmChartProxy when it is not being deleted. It takes a list of selected Clusters and HelmReleaseProxies
// to uninstall the Helm chart from any Clusters that are no longer selected and to install or update the Helm chart on any Clusters that currently selected.
func (r *HelmChartProxyReconciler) reconcileNormal(ctx context.Context, helmChartProxy *addonsv1alpha1.HelmChartProxy, clusters []clusterv1.Cluster, helmReleaseProxies []addonsv1alpha1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Starting reconcileNormal for chart proxy", "name", helmChartProxy.Name)

	err := r.deleteOrphanedHelmReleaseProxies(ctx, helmChartProxy, clusters, helmReleaseProxies)
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		// Don't reconcile if the Cluster is being deleted
		if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
			continue
		}

		err := r.reconcileForCluster(ctx, helmChartProxy, cluster)
		if err != nil {
			return err
		}
	}

	return nil
}

// reconcileDelete handles the deletion of a HelmChartProxy. It takes a list of HelmReleaseProxies to uninstall the Helm chart from all selected Clusters.
func (r *HelmChartProxyReconciler) reconcileDelete(ctx context.Context, helmChartProxy *addonsv1alpha1.HelmChartProxy, releases []addonsv1alpha1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Deleting all HelmReleaseProxies as part of HelmChartProxy deletion", "helmChartProxy", helmChartProxy.Name)

	for i := range releases {
		release := releases[i]

		log.V(2).Info("Deleting release", "releaseName", release.Name, "cluster", release.Spec.ClusterRef.Name)
		if err := r.deleteHelmReleaseProxy(ctx, &release); err != nil {
			// TODO: will this fail if clusterRef is nil
			return errors.Wrapf(err, "failed to delete release %s from cluster %s", release.Name, release.Spec.ClusterRef.Name)
		}
	}

	return nil
}

// listClustersWithLabels returns a list of Clusters that match the given label selector.
func (r *HelmChartProxyReconciler) listClustersWithLabels(ctx context.Context, namespace string, selector metav1.LabelSelector) (*clusterv1.ClusterList, error) {
	clusterList := &clusterv1.ClusterList{}
	// To support for the matchExpressions field, convert LabelSelector to labels.Selector to specify labels.Selector for ListOption. (Issue #15)
	labelselector, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil {
		return nil, err
	}

	if err := r.Client.List(ctx, clusterList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: labelselector}); err != nil {
		return nil, err
	}

	return clusterList, nil
}

// listInstalledReleases returns a list of HelmReleaseProxies that match the given label selector.
func (r *HelmChartProxyReconciler) listInstalledReleases(ctx context.Context, namespace string, labels map[string]string) (*addonsv1alpha1.HelmReleaseProxyList, error) {
	releaseList := &addonsv1alpha1.HelmReleaseProxyList{}

	// TODO: should we use client.MatchingLabels or try to use the labelSelector itself?
	if err := r.Client.List(ctx, releaseList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return releaseList, nil
}

// aggregateHelmReleaseProxyReadyCondition HelmReleaseProxyReadyCondition from all HelmReleaseProxies that match the given label selector.
func (r *HelmChartProxyReconciler) aggregateHelmReleaseProxyReadyCondition(ctx context.Context, helmChartProxy *addonsv1alpha1.HelmChartProxy) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Aggregating HelmReleaseProxyReadyCondition")

	labels := map[string]string{
		addonsv1alpha1.HelmChartProxyLabelName: helmChartProxy.Name,
	}
	releaseList, err := r.listInstalledReleases(ctx, helmChartProxy.Namespace, labels)
	if err != nil {
		// conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxiesReadyCondition, addonsv1alpha1.HelmReleaseProxyListFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return err
	}

	if len(releaseList.Items) == 0 {
		// Consider it to be vacuously true if there are no releases. This should only be reached if we previously had HelmReleaseProxies but they were all deleted
		// due to the Clusters being unselected. In that case, we should consider the condition to be true.
		conditions.MarkTrue(helmChartProxy, addonsv1alpha1.HelmReleaseProxiesReadyCondition)
		return nil
	}

	getters := make([]conditions.Getter, 0, len(releaseList.Items))
	for i := range releaseList.Items {
		helmReleaseProxy := &releaseList.Items[i]
		if helmReleaseProxy.Generation != helmReleaseProxy.Status.ObservedGeneration {
			conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition, addonsv1alpha1.HelmReleaseProxySpecsUpdatingReason, clusterv1.ConditionSeverityInfo, "Helm release proxy '%s' is not updated yet", helmReleaseProxy.Name)
			return nil
		}
		getters = append(getters, helmReleaseProxy)
	}

	conditions.SetAggregate(helmChartProxy, addonsv1alpha1.HelmReleaseProxiesReadyCondition, getters, conditions.AddSourceRef(), conditions.WithStepCounterIf(false))

	return nil
}

// patchHelmChartProxy patches the HelmChartProxy object and sets the ReadyCondition as an aggregate of the other condition set.
// TODO: Is this preferable to client.Update() calls? Based on testing it seems like it avoids race conditions.
func patchHelmChartProxy(ctx context.Context, patchHelper *patch.Helper, helmChartProxy *addonsv1alpha1.HelmChartProxy) error {
	conditions.SetSummary(helmChartProxy,
		conditions.WithConditions(
			addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition,
			addonsv1alpha1.HelmReleaseProxiesReadyCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		helmChartProxy,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition,
			addonsv1alpha1.HelmReleaseProxiesReadyCondition,
		}},
		patch.WithStatusObservedGeneration{},
	)
}

// ClusterToHelmChartProxiesMapper is a mapper function that maps a Cluster to the HelmChartProxies that would select the Cluster.
func (r *HelmChartProxyReconciler) ClusterToHelmChartProxiesMapper(ctx context.Context, o client.Object) []ctrl.Request {
	log := ctrl.LoggerFrom(ctx)

	cluster, ok := o.(*clusterv1.Cluster)
	if !ok {
		// Suppress the error for now
		log.Error(errors.Errorf("expected a Cluster but got %T", o), "failed to map object to HelmChartProxy")
		return nil
	}

	helmChartProxies := &addonsv1alpha1.HelmChartProxyList{}

	// TODO: Figure out if we want this search to be cross-namespaces.

	if err := r.Client.List(ctx, helmChartProxies, client.InNamespace(cluster.Namespace)); err != nil {
		return nil
	}

	results := []ctrl.Request{}
	for _, helmChartProxy := range helmChartProxies.Items {
		selector, err := metav1.LabelSelectorAsSelector(&helmChartProxy.Spec.ClusterSelector)
		if err != nil {
			// Suppress the error for now
			log.Error(err, "failed to parse ClusterSelector for HelmChartProxy", "helmChartProxy", helmChartProxy.Name)
			return nil
		}

		if selector.Matches(labels.Set(cluster.Labels)) {
			results = append(results, ctrl.Request{
				// The HelmReleaseProxy is always in the same namespace as the HelmChartProxy.
				NamespacedName: client.ObjectKey{Namespace: helmChartProxy.Namespace, Name: helmChartProxy.Name},
			})
		}
	}

	return results
}

// HelmReleaseProxyToHelmChartProxyMapper is a mapper function that maps a HelmReleaseProxy to the HelmChartProxy that owns it.
// This is used to trigger an update of the HelmChartProxy when a HelmReleaseProxy is changed.
func HelmReleaseProxyToHelmChartProxyMapper(ctx context.Context, o client.Object) []ctrl.Request {
	log := ctrl.LoggerFrom(ctx)

	helmReleaseProxy, ok := o.(*addonsv1alpha1.HelmReleaseProxy)
	if !ok {
		// Suppress the error for now
		log.Error(errors.Errorf("expected a HelmReleaseProxy but got %T", o), "failed to map object to HelmChartProxy")
		return nil
	}

	// Check if the controller reference is already set and
	// return an empty result when one is found.
	for _, ref := range helmReleaseProxy.ObjectMeta.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			name := client.ObjectKey{
				Namespace: helmReleaseProxy.GetNamespace(),
				Name:      ref.Name,
			}

			return []ctrl.Request{
				{
					NamespacedName: name,
				},
			}
		}
	}

	return nil
}
