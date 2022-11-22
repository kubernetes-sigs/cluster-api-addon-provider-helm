/*
Copyright 2022.

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
	"fmt"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	addonsv1alpha1 "cluster-api-addon-provider-helm/api/v1alpha1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/cluster"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
)

// HelmChartProxyReconciler reconciles a HelmChartProxy object
type HelmChartProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmChartProxyReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&addonsv1alpha1.HelmChartProxy{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		// WithEventFilter(predicates.ResourceIsNotExternallyManaged(log)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for changes.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.ClusterToHelmChartProxiesMapper),
		predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for Clusters")
	}

	// Add a watch on HelmReleaseProxy object for changes.
	if err = c.Watch(
		&source.Kind{Type: &addonsv1alpha1.HelmReleaseProxy{}},
		handler.EnqueueRequestsFromMapFunc(HelmReleaseProxyToHelmChartProxyMapper),
		predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for HelmReleaseProxies")
	}

	return nil
}

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/finalizers,verbs=update
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=list;watch
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=list;get;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io,resources=*,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=list;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HelmChartProxy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *HelmChartProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Beginning reconcilation for HelmChartProxy", "requestNamespace", req.Namespace, "requestName", req.Name)

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
		conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition, addonsv1alpha1.ClusterSelectionFailedReason, clusterv1.ConditionSeverityError, err.Error())

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

// reconcileNormal...
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

// reconcileDelete...
func (r *HelmChartProxyReconciler) reconcileDelete(ctx context.Context, helmChartProxy *addonsv1alpha1.HelmChartProxy, releases []addonsv1alpha1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Deleting all HelmReleaseProxies as part of HelmChartProxy deletion", "helmChartProxy", helmChartProxy.Name)

	for _, release := range releases {
		log.V(2).Info("Deleting release", "releaseName", release.Name, "cluster", release.Spec.ClusterRef.Name)
		if err := r.deleteHelmReleaseProxy(ctx, &release); err != nil {
			// TODO: will this fail if clusterRef is nil
			return errors.Wrapf(err, "failed to delete release %s from cluster %s", release.Name, release.Spec.ClusterRef.Name)
		}
	}

	return nil
}

func (r *HelmChartProxyReconciler) listClustersWithLabels(ctx context.Context, namespace string, selector metav1.LabelSelector) (*clusterv1.ClusterList, error) {
	clusterList := &clusterv1.ClusterList{}
	// TODO: validate empty key or empty value to make sure it doesn't match everything.
	if err := r.Client.List(ctx, clusterList, client.InNamespace(namespace), client.MatchingLabels(selector.MatchLabels)); err != nil {
		return nil, err
	}

	return clusterList, nil
}

func (r *HelmChartProxyReconciler) listInstalledReleases(ctx context.Context, namespace string, labels map[string]string) (*addonsv1alpha1.HelmReleaseProxyList, error) {
	releaseList := &addonsv1alpha1.HelmReleaseProxyList{}
	// Empty labels should match nothing, not everything
	if len(labels) == 0 {
		return nil, nil
	}

	// TODO: should we use client.MatchingLabels or try to use the labelSelector itself?
	if err := r.Client.List(ctx, releaseList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return releaseList, nil
}

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
	for _, r := range releaseList.Items {
		getters = append(getters, &r)
	}

	conditions.SetAggregate(helmChartProxy, addonsv1alpha1.HelmReleaseProxiesReadyCondition, getters, conditions.AddSourceRef(), conditions.WithStepCounterIf(false))

	return nil
}

func patchHelmChartProxy(ctx context.Context, patchHelper *patch.Helper, helmChartProxy *addonsv1alpha1.HelmChartProxy) error {
	// TODO: Update the readyCondition by summarizing the state of other conditions when they are implemented.
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

// Note: this finds every HelmReleaseProxy associated with a Cluster and returns a Request for its parent HelmChartProxy.
// This will not trigger an update if the HelmChartProxy selected a Cluster but ran into an error before creating the HelmReleaseProxy.
// Though in that case the HelmChartProxy will requeue soon anyway so it's most likely not an issue.
func (r *HelmChartProxyReconciler) ClusterToHelmChartProxiesMapper(o client.Object) []ctrl.Request {
	cluster, ok := o.(*clusterv1.Cluster)
	if !ok {
		// Suppress the error for now
		fmt.Printf("Expected a Cluster but got %T\n", o)
		return nil
	}

	helmReleaseProxies := &addonsv1alpha1.HelmReleaseProxyList{}

	listOpts := []client.ListOption{
		client.MatchingLabels{
			clusterv1.ClusterLabelName: cluster.Name,
		},
	}

	// TODO: Figure out if we want this search to be cross-namespaces.

	if err := r.Client.List(context.TODO(), helmReleaseProxies, listOpts...); err != nil {
		return nil
	}

	results := []ctrl.Request{}
	for _, helmReleaseProxy := range helmReleaseProxies.Items {
		results = append(results, ctrl.Request{
			// The HelmReleaseProxy is always in the same namespace as the HelmChartProxy.
			NamespacedName: client.ObjectKey{Namespace: helmReleaseProxy.GetNamespace(), Name: helmReleaseProxy.Labels[addonsv1alpha1.HelmChartProxyLabelName]},
		})
	}

	return results
}

func HelmReleaseProxyToHelmChartProxyMapper(o client.Object) []ctrl.Request {
	helmReleaseProxy, ok := o.(*addonsv1alpha1.HelmReleaseProxy)
	if !ok {
		// Suppress the error for now
		fmt.Printf("Expected a HelmReleaseProxy but got %T\n", o)
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
