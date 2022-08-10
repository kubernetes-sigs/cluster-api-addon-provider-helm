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

package controllers

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	addonsv1beta1 "cluster-api-addon-provider-helm/api/v1beta1"
	"cluster-api-addon-provider-helm/internal"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
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

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/finalizers,verbs=update
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=list;watch
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=list;get;watch

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
	helmChartProxy := &addonsv1beta1.HelmChartProxy{}
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

	label := helmChartProxy.Spec.ClusterSelector

	log.V(2).Info("Finding matching clusters for HelmChartProxy with label", "helmChartProxy", helmChartProxy.Name, "label", label)
	// TODO: When a Cluster is being deleted, it will show up in the list of clusters even though we can't Reconcile on it.
	// This is because of ownerRefs and how the Cluster gets deleted. It will be eventually consistent but it would be better
	// to not have errors. An idea would be to check the deletion timestamp.
	clusterList, err := r.listClustersWithLabel(ctx, label)
	if err != nil {
		helmChartProxy.SetError(errors.Wrapf(err, "failed to list clusters with label selector %+v", label))
		return ctrl.Result{}, err
	}
	helmChartProxy.SetMatchingClusters(clusterList.Items)

	log.V(2).Info("Finding HelmRelease for HelmChartProxy", "helmChartProxy", helmChartProxy.Name)
	labels := map[string]string{
		addonsv1beta1.HelmChartProxyLabelName: helmChartProxy.Name,
	}
	releaseList, err := r.listInstalledReleases(ctx, labels)
	if err != nil {
		helmChartProxy.SetError(errors.Wrapf(err, "failed to list installed releases with labels %+v", labels))
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if helmChartProxy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(helmChartProxy, addonsv1beta1.HelmChartProxyFinalizer) {
			controllerutil.AddFinalizer(helmChartProxy, addonsv1beta1.HelmChartProxyFinalizer)
			if err := r.Update(ctx, helmChartProxy); err != nil {
				// TODO: Should we try to set the error here? If we can't add the finalizer we likely can't update the status either.
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(helmChartProxy, addonsv1beta1.HelmChartProxyFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.reconcileDelete(ctx, helmChartProxy, releaseList.Items); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				helmChartProxy.SetError(err)
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(helmChartProxy, addonsv1beta1.HelmChartProxyFinalizer)
			if err := r.Update(ctx, helmChartProxy); err != nil {
				// TODO: Should we try to set the error here? If we can't remove the finalizer we likely can't update the status either.
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		helmChartProxy.SetError(nil)
		return ctrl.Result{}, nil
	}

	log.V(2).Info("Reconciling HelmChartProxy", "randomName", helmChartProxy.Name)
	err = r.reconcileNormal(ctx, helmChartProxy, clusterList.Items, releaseList.Items)

	helmChartProxy.SetError(err)
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmChartProxyReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)

	helmChartProxyMapper, err := ClusterToHelmChartProxiesMapper(r.Client)
	if err != nil {
		return errors.Wrap(err, "failed to create mapper for Cluster to HelmChartProxies")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&addonsv1beta1.HelmChartProxy{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		// WithEventFilter(predicates.ResourceIsNotExternallyManaged(log)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for changes.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(helmChartProxyMapper),
		predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for clusters")
	}

	return nil
}

// reconcileNormal...
func (r *HelmChartProxyReconciler) reconcileNormal(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, clusters []clusterv1.Cluster, helmReleaseProxies []addonsv1beta1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Starting reconcileNormal for chart proxy", "name", helmChartProxy.Name)

	releasesToDelete := getOrphanedHelmReleaseProxies(ctx, clusters, helmReleaseProxies)
	log.V(2).Info("Deleting orphaned releases")
	for _, release := range releasesToDelete {
		log.V(2).Info("Deleting release", "release", release)
		if err := r.deleteHelmReleaseProxy(ctx, &release); err != nil {
			return err
		}
	}

	for _, cluster := range clusters {
		existingHelmReleaseProxy, err := r.getExistingHelmReleaseProxy(ctx, helmChartProxy, &cluster)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("HelmReleaseProxy for cluster not found", "cluster", cluster.Name)
			} else {
				return errors.Wrapf(err, "failed to get HelmReleaseProxy for cluster %s", cluster.Name)
			}
		}
		// log.V(2).Info("Found existing HelmReleaseProxy", "cluster", cluster.Name, "release", existingHelmReleaseProxy.Name)

		if existingHelmReleaseProxy != nil && shouldReinstallHelmRelease(ctx, existingHelmReleaseProxy, helmChartProxy) {
			log.V(2).Info("Reinstalling Helm release by deleting and creating HelmReleaseProxy", "helmReleaseProxy", existingHelmReleaseProxy.Name)
			if err := r.deleteHelmReleaseProxy(ctx, existingHelmReleaseProxy); err != nil {
				return err
			}

			// TODO: Add a check on requeue to make sure that the HelmReleaseProxy isn't still deleting
			log.V(2).Info("Successfully deleted HelmReleaseProxy on cluster, returning to requeue for reconcile", "cluster", cluster.Name)
			return nil // Try returning early so it will requeue
			// existingHelmReleaseProxy = nil // Set to nil so we can create a new one
		}

		values, err := internal.ParseValues(ctx, r.Client, helmChartProxy.Spec, &cluster)
		if err != nil {
			return errors.Wrapf(err, "failed to parse values on cluster %s", cluster.Name)
		}

		log.V(2).Info("Values for cluster", "cluster", cluster.Name, "values", values)
		if err := r.createOrUpdateHelmReleaseProxy(ctx, existingHelmReleaseProxy, helmChartProxy, &cluster, values); err != nil {
			return errors.Wrapf(err, "failed to create or update HelmReleaseProxy on cluster %s", cluster.Name)
		}
	}

	return nil
}

func getOrphanedHelmReleaseProxies(ctx context.Context, clusters []clusterv1.Cluster, helmReleaseProxies []addonsv1beta1.HelmReleaseProxy) []addonsv1beta1.HelmReleaseProxy {
	log := ctrl.LoggerFrom(ctx)
	log.V(2).Info("Getting HelmReleaseProxies to delete")

	selectedClusters := map[string]struct{}{}
	for _, cluster := range clusters {
		key := cluster.GetNamespace() + "/" + cluster.GetName()
		selectedClusters[key] = struct{}{}
	}
	log.V(2).Info("Selected clusters", "clusters", selectedClusters)

	releasesToDelete := []addonsv1beta1.HelmReleaseProxy{}
	for _, helmReleaseProxy := range helmReleaseProxies {
		clusterRef := helmReleaseProxy.Spec.ClusterRef
		if clusterRef != nil {
			key := clusterRef.Namespace + "/" + clusterRef.Name
			if _, ok := selectedClusters[key]; !ok {
				releasesToDelete = append(releasesToDelete, helmReleaseProxy)
			}
		}
	}

	names := make([]string, len(releasesToDelete))
	for _, release := range releasesToDelete {
		names = append(names, release.Name)
	}
	log.V(2).Info("Releases to delete", "releases", names)

	return releasesToDelete
}

// reconcileDelete...
func (r *HelmChartProxyReconciler) reconcileDelete(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, releases []addonsv1beta1.HelmReleaseProxy) error {
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

func (r *HelmChartProxyReconciler) listClustersWithLabel(ctx context.Context, label addonsv1beta1.ClusterSelectorLabel) (*clusterv1.ClusterList, error) {
	clusterList := &clusterv1.ClusterList{}
	// TODO: validate empty key or empty value to make sure it doesn't match everything.
	labelMap := map[string]string{
		label.Key: label.Value,
	}

	if err := r.Client.List(ctx, clusterList, client.MatchingLabels(labelMap)); err != nil {
		return nil, err
	}

	return clusterList, nil
}

func (r *HelmChartProxyReconciler) listInstalledReleases(ctx context.Context, labels map[string]string) (*addonsv1beta1.HelmReleaseProxyList, error) {
	releaseList := &addonsv1beta1.HelmReleaseProxyList{}
	// Empty labels should match nothing, not everything
	if len(labels) == 0 {
		return nil, nil
	}

	// TODO: should we use client.MatchingLabels or try to use the labelSelector itself?
	if err := r.Client.List(ctx, releaseList, client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return releaseList, nil
}

// getExistingHelmReleaseProxy...
func (r *HelmChartProxyReconciler) getExistingHelmReleaseProxy(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, cluster *clusterv1.Cluster) (*addonsv1beta1.HelmReleaseProxy, error) {
	log := ctrl.LoggerFrom(ctx)
	helmReleaseProxyName := generateHelmReleaseProxyName(*helmChartProxy, *cluster)
	helmReleaseProxyNamespace := helmChartProxy.Namespace

	existing := &addonsv1beta1.HelmReleaseProxy{}
	helmReleaseProxyKey := client.ObjectKey{
		Namespace: helmReleaseProxyNamespace,
		Name:      helmReleaseProxyName,
	}

	log.V(2).Info("Getting HelmReleaseProxy", "cluster", cluster.Name)
	err := r.Client.Get(ctx, helmReleaseProxyKey, existing)
	if err != nil {
		return nil, err
	}

	return existing, nil
}

// createOrUpdateHelmReleaseProxy...
func (r *HelmChartProxyReconciler) createOrUpdateHelmReleaseProxy(ctx context.Context, existing *addonsv1beta1.HelmReleaseProxy, helmChartProxy *addonsv1beta1.HelmChartProxy, cluster *clusterv1.Cluster, parsedValues string) error {
	log := ctrl.LoggerFrom(ctx)
	helmReleaseProxyName := generateHelmReleaseProxyName(*helmChartProxy, *cluster)

	helmReleaseProxy := constructHelmReleaseProxy(helmReleaseProxyName, existing, helmChartProxy, parsedValues, cluster)
	if helmReleaseProxy == nil {
		log.V(2).Info("HelmReleaseProxy is up to date, nothing to do", "helmReleaseProxy", existing.Name, "cluster", cluster.Name)
		return nil
	}
	if existing == nil {
		if err := r.Client.Create(ctx, helmReleaseProxy); err != nil {
			return errors.Wrapf(err, "failed to create HelmReleaseProxy '%s' for cluster: %s/%s", helmReleaseProxy.Name, cluster.Namespace, cluster.Name)
		}
	} else {
		if err := r.Client.Update(ctx, helmReleaseProxy); err != nil {
			return errors.Wrapf(err, "failed to update HelmReleaseProxy '%s' for cluster: %s/%s", helmReleaseProxy.Name, cluster.Namespace, cluster.Name)
		}
	}

	return nil
}

// deleteHelmReleaseProxy...
func (r *HelmChartProxyReconciler) deleteHelmReleaseProxy(ctx context.Context, helmReleaseProxy *addonsv1beta1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	if err := r.Client.Delete(ctx, helmReleaseProxy); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("HelmReleaseProxy already deleted, nothing to do", "helmReleaseProxy", helmReleaseProxy.Name)
			return nil
		}
		return errors.Wrapf(err, "failed to delete helmReleaseProxy: %s", helmReleaseProxy.Name)
	}

	return nil
}

func constructHelmReleaseProxy(name string, existing *addonsv1beta1.HelmReleaseProxy, helmChartProxy *addonsv1beta1.HelmChartProxy, parsedValues string, cluster *clusterv1.Cluster) *addonsv1beta1.HelmReleaseProxy {
	helmReleaseProxy := &addonsv1beta1.HelmReleaseProxy{}
	if existing == nil {
		helmReleaseProxy.Name = name
		helmReleaseProxy.Namespace = helmChartProxy.Namespace
		helmReleaseProxy.OwnerReferences = util.EnsureOwnerRef(helmReleaseProxy.OwnerReferences, *metav1.NewControllerRef(helmChartProxy, helmChartProxy.GroupVersionKind()))
		newLabels := map[string]string{}
		newLabels[clusterv1.ClusterLabelName] = cluster.Name
		newLabels[addonsv1beta1.HelmChartProxyLabelName] = helmChartProxy.Name
		helmReleaseProxy.Labels = newLabels

		helmReleaseProxy.Spec.ClusterRef = &corev1.ObjectReference{
			Kind:       cluster.Kind,
			APIVersion: cluster.APIVersion,
			Name:       cluster.Name,
			Namespace:  cluster.Namespace,
		}
		// helmChartProxy.ObjectMeta.SetAnnotations(helmReleaseProxy.Annotations)
	} else {
		helmReleaseProxy = existing
		changed := false
		if existing.Spec.Version != helmChartProxy.Spec.Version {
			changed = true
		}
		if !cmp.Equal(existing.Spec.Values, parsedValues) {
			changed = true
		}

		if !changed {
			return nil
		}
	}

	helmReleaseProxy.Spec.ChartName = helmChartProxy.Spec.ChartName
	helmReleaseProxy.Spec.RepoURL = helmChartProxy.Spec.RepoURL
	helmReleaseProxy.Spec.ReleaseName = helmChartProxy.Spec.ReleaseName
	helmReleaseProxy.Spec.Version = helmChartProxy.Spec.Version
	helmReleaseProxy.Spec.Namespace = helmChartProxy.Spec.Namespace
	helmReleaseProxy.Spec.Values = parsedValues

	return helmReleaseProxy
}

func shouldReinstallHelmRelease(ctx context.Context, existing *addonsv1beta1.HelmReleaseProxy, helmChartProxy *addonsv1beta1.HelmChartProxy) bool {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Checking if HelmReleaseProxy needs to be reinstalled by by checking if immutable fields changed", "helmReleaseProxy", existing.Name)

	annotations := existing.GetAnnotations()
	result, ok := annotations[addonsv1beta1.IsReleaseNameGeneratedAnnotation]

	// log.V(2).Info("IsReleaseNameGeneratedAnnotation", "result", result, "ok", ok)

	isReleaseNameGenerated := ok && result == "true"
	switch {
	case existing.Spec.ChartName != helmChartProxy.Spec.ChartName:
		log.V(2).Info("ChartName changed", "existing", existing.Spec.ChartName, "helmChartProxy", helmChartProxy.Spec.ChartName)
	case existing.Spec.RepoURL != helmChartProxy.Spec.RepoURL:
		log.V(2).Info("RepoURL changed", "existing", existing.Spec.RepoURL, "helmChartProxy", helmChartProxy.Spec.RepoURL)
	case isReleaseNameGenerated && helmChartProxy.Spec.ReleaseName != "":
		log.V(2).Info("Generated ReleaseName changed", "existing", existing.Spec.ReleaseName, "helmChartProxy", helmChartProxy.Spec.ReleaseName)
	case !isReleaseNameGenerated && existing.Spec.ReleaseName != helmChartProxy.Spec.ReleaseName:
		log.V(2).Info("Non-generated ReleaseName changed", "existing", existing.Spec.ReleaseName, "helmChartProxy", helmChartProxy.Spec.ReleaseName)
	case existing.Spec.Namespace != helmChartProxy.Spec.Namespace:
		log.V(2).Info("Namespace changed", "existing", existing.Spec.Namespace, "helmChartProxy", helmChartProxy.Spec.Namespace)
		return true
	}

	return false
}

func generateHelmReleaseProxyName(helmChartProxy addonsv1beta1.HelmChartProxy, cluster clusterv1.Cluster) string {
	return helmChartProxy.Spec.ChartName + "-" + cluster.Name
}

func patchHelmChartProxy(ctx context.Context, patchHelper *patch.Helper, helmChartProxy *addonsv1beta1.HelmChartProxy) error {
	// TODO: Update the readyCondition by summarizing the state of other conditions when they are implemented.
	conditions.SetSummary(helmChartProxy,
		conditions.WithConditions(
			clusterv1.ReadyCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		helmChartProxy,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
		}},
		patch.WithStatusObservedGeneration{},
	)
}

func ClusterToHelmChartProxiesMapper(c client.Client) (handler.MapFunc, error) {
	// Note: this finds every HelmReleaseProxy associated with a Cluster and returns a Request for its parent HelmChartProxy.
	// This will not trigger an update if the HelmChartProxy selected a Cluster but ran into an error before creating the HelmReleaseProxy.
	// Though in that case the HelmChartProxy will requeue soon anyway so it's most likely not an issue.
	return func(o client.Object) []ctrl.Request {
		cluster, ok := o.(*clusterv1.Cluster)
		if !ok {
			return nil
		}

		helmReleaseProxies := &addonsv1beta1.HelmReleaseProxyList{}

		listOpts := []client.ListOption{
			client.MatchingLabels{
				clusterv1.ClusterLabelName: cluster.Name,
			},
		}

		// TODO: Figure out if we want this search to be cross-namespaces.

		if err := c.List(context.TODO(), helmReleaseProxies, listOpts...); err != nil {
			return nil
		}

		results := []ctrl.Request{}
		for _, helmReleaseProxy := range helmReleaseProxies.Items {
			results = append(results, ctrl.Request{
				// The HelmReleaseProxy is always in the same namespace as the HelmChartProxy.
				NamespacedName: client.ObjectKey{Namespace: helmReleaseProxy.GetNamespace(), Name: helmReleaseProxy.Labels[addonsv1beta1.HelmChartProxyLabelName]},
			})
		}

		return results
	}, nil
}
