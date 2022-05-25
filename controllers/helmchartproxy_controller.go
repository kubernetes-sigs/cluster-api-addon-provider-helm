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

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"
	"cluster-api-addon-helm/internal"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/cluster"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
)

// HelmChartProxyReconciler reconciles a HelmChartProxy object
type HelmChartProxyReconciler struct {
	ctrlClient.Client
	Scheme *runtime.Scheme
}

const finalizer = "addons.cluster.x-k8s.io"
const HelmChartProxyLabelName = "addons.cluster.x-k8s.io/helmchartproxy-name"

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/finalizers,verbs=update

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
		if err := patchHelper.Patch(ctx, helmChartProxy); err != nil && reterr == nil {
			reterr = err
			log.Error(err, "failed to patch HelmChartProxy", "helmChartProxy", helmChartProxy.Name)
			return
		}
		log.V(2).Info("Successfully patched HelmChartProxy", "helmChartProxy", helmChartProxy.Name)
	}()

	label := helmChartProxy.Spec.ClusterSelector
	log.V(2).Info("HelmChartProxy labels are", "labels", label)

	log.V(2).Info("Getting list of clusters with labels")
	clusterList, err := r.listClustersWithLabel(ctx, label)
	if err != nil {
		helmChartProxy.Status.FailureReason = to.StringPtr((errors.Wrapf(err, "failed to list clusters with label selector %+v", label).Error()))
		helmChartProxy.Status.Ready = false

		return ctrl.Result{}, err
	}
	if clusterList == nil {
		log.V(2).Info("No clusters found")
	}
	for _, cluster := range clusterList.Items {
		log.V(2).Info("Found cluster", "name", cluster.Name)
	}

	log.V(2).Info("Getting list of HelmReleaseProxies with label matching HelmChartProxy name")
	labels := map[string]string{
		HelmChartProxyLabelName: helmChartProxy.Name,
	}
	releaseList, err := r.listInstalledReleases(ctx, labels)
	if releaseList == nil {
		log.V(2).Info("No HelmReleaseProxy found")
	}
	for _, release := range releaseList.Items {
		log.V(2).Info("Found HelmReleaseProxy", "name", release.Name)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if helmChartProxy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(helmChartProxy, finalizer) {
			controllerutil.AddFinalizer(helmChartProxy, finalizer)
			if err := r.Update(ctx, helmChartProxy); err != nil {
				helmChartProxy.Status.FailureReason = to.StringPtr(errors.Wrapf(err, "failed to add finalizer").Error())
				helmChartProxy.Status.Ready = false
				if err := r.Status().Update(ctx, helmChartProxy); err != nil {
					log.Error(err, "unable to update HelmChartProxy status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(helmChartProxy, finalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.reconcileDelete(ctx, helmChartProxy, releaseList.Items); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				helmChartProxy.Status.FailureReason = to.StringPtr(err.Error())
				helmChartProxy.Status.Ready = false
				if err := r.Status().Update(ctx, helmChartProxy); err != nil {
					log.Error(err, "unable to update HelmChartProxy status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}

			helmChartProxy.Status.Ready = true
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(helmChartProxy, finalizer)
			if err := r.Update(ctx, helmChartProxy); err != nil {
				helmChartProxy.Status.FailureReason = to.StringPtr(errors.Wrapf(err, "failed to remove finalizer").Error())
				helmChartProxy.Status.Ready = false
				if err := r.Status().Update(ctx, helmChartProxy); err != nil {
					log.Error(err, "unable to update HelmChartProxy status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	log.V(2).Info("Reconciling HelmChartProxy", "randomName", helmChartProxy.Name)
	err = r.reconcileNormal(ctx, helmChartProxy, clusterList.Items, releaseList.Items)
	if err != nil {
		helmChartProxy.Status.Ready = false

		return ctrl.Result{}, err
	}

	helmChartProxy.Status.FailureReason = nil
	helmChartProxy.Status.Ready = true
	if err := r.Status().Update(ctx, helmChartProxy); err != nil {
		log.Error(err, "unable to update HelmChartProxy status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmChartProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1beta1.HelmChartProxy{}).
		// Watches(
		// 	&source.Kind{Type: &v1beta1.HelmChartProxy{}},
		// 	handler.EnqueueRequestsFromMapFunc(r.findProxyForSecret),
		// 	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		// ).
		Complete(r)
}

// reconcileNormal...
func (r *HelmChartProxyReconciler) reconcileNormal(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, clusters []clusterv1.Cluster, helmReleaseProxies []addonsv1beta1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Starting reconcileNormal for chart proxy", "name", helmChartProxy.Name)

	if len(clusters) == 0 {
		log.V(2).Info("No clusters to reconcile")

		return nil
	}

	releasesToDelete := getHelmReleaseProxiesToDelete(ctx, clusters, helmReleaseProxies)
	log.V(2).Info("Deleting orphaned releases")
	for _, release := range releasesToDelete {
		log.V(2).Info("Deleting release", "release", release)
		if err := r.deleteHelmReleaseProxy(ctx, &release); err != nil {
			return err
		}
	}

	for _, cluster := range clusters {
		log.V(2).Info("Creating or updating on cluster", "cluster", cluster.Name)
		kubeconfigPath, err := internal.WriteClusterKubeconfigToFile(ctx, &cluster)
		if err != nil {
			log.Error(err, "failed to get kubeconfig for cluster", "cluster", cluster.Name)
			return err
		}

		values, err := internal.ParseValues(ctx, r.Client, kubeconfigPath, helmChartProxy.Spec, &cluster)
		if err != nil {
			return errors.Wrapf(err, "failed to parse values on cluster %s", cluster.Name)
		}

		log.V(2).Info("Values for cluster", "cluster", cluster.Name, "values", values)
		if err := r.createOrUpdateHelmReleaseProxy(ctx, helmChartProxy, &cluster, values); err != nil {
			return errors.Wrapf(err, "failed to create or update HelmReleaseProxy on cluster %s", cluster.Name)
		}
	}

	return nil
}

func getHelmReleaseProxiesToDelete(ctx context.Context, clusters []clusterv1.Cluster, helmReleaseProxies []addonsv1beta1.HelmReleaseProxy) []addonsv1beta1.HelmReleaseProxy {
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

	if err := r.Client.List(ctx, clusterList, ctrlClient.MatchingLabels(labelMap)); err != nil {
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

	// TODO: should we use ctrlClient.MatchingLabels or try to use the labelSelector itself?
	if err := r.Client.List(ctx, releaseList, ctrlClient.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return releaseList, nil
}

// createOrUpdateHelmReleaseProxy...
func (r *HelmChartProxyReconciler) createOrUpdateHelmReleaseProxy(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, cluster *clusterv1.Cluster, parsedValues map[string]string) error {
	log := ctrl.LoggerFrom(ctx)
	helmReleaseProxyName := helmChartProxy.Spec.ReleaseName + "-" + cluster.Name
	helmReleaseProxyNamespace := helmChartProxy.Namespace

	existing := &addonsv1beta1.HelmReleaseProxy{}
	helmReleaseProxyKey := client.ObjectKey{
		Namespace: helmReleaseProxyNamespace,
		Name:      helmReleaseProxyName,
	}

	log.V(2).Info("Getting HelmReleaseProxy", "cluster", cluster.Name)
	if err := r.Client.Get(ctx, helmReleaseProxyKey, existing); err != nil {
		log.Error(err, "failed to get HelmReleaseProxy", "cluster", cluster.Name)
		if !apierrors.IsNotFound(err) {
			return err
		}

		helmReleaseProxy := constructHelmReleaseProxy(helmReleaseProxyName, nil, helmChartProxy, parsedValues, cluster)

		if err := r.Client.Create(ctx, helmReleaseProxy); err != nil {
			return errors.Wrapf(err, "failed to create helmReleaseProxy for cluster: %s/%s", cluster.Namespace, cluster.Name)
		}
	} else {
		log.V(2).Info("Existing HelmReleaseValues are", "releaseName", existing.Name, "values", existing.Spec.Values)
		helmReleaseProxy := constructHelmReleaseProxy(helmReleaseProxyName, existing, helmChartProxy, parsedValues, cluster)
		if helmReleaseProxy == nil {
			log.V(2).Info("HelmReleaseProxy is up to date, nothing to do", "helmReleaseProxy", existing.Name, "cluster", cluster.Name)
		}
		if err := r.Client.Update(ctx, helmReleaseProxy); err != nil {
			return errors.Wrapf(err, "failed to update helmReleaseProxy for cluster: %s/%s", cluster.Namespace, cluster.Name)
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
		return errors.Wrapf(err, "failed to create helmReleaseProxy: %s", helmReleaseProxy.Name)
	}

	return nil
}

func constructHelmReleaseProxy(name string, existing *addonsv1beta1.HelmReleaseProxy, helmChartProxy *addonsv1beta1.HelmChartProxy, parsedValues map[string]string, cluster *clusterv1.Cluster) *addonsv1beta1.HelmReleaseProxy {
	helmReleaseProxy := &addonsv1beta1.HelmReleaseProxy{}
	if existing == nil {
		helmReleaseProxy.Name = name
		helmReleaseProxy.Namespace = helmChartProxy.Namespace
		// helmReleaseProxy.OwnerReferences = util.EnsureOwnerRef(helmReleaseProxy.OwnerReferences, metav1.OwnerReference{
		// 	APIVersion: clusterv1.GroupVersion.String(),
		// 	Kind:       "Cluster",
		// 	Name:       cluster.Name,
		// 	UID:        cluster.UID,
		// })
		helmReleaseProxy.OwnerReferences = util.EnsureOwnerRef(helmReleaseProxy.OwnerReferences, *metav1.NewControllerRef(helmChartProxy, helmChartProxy.GroupVersionKind()))
		newLabels := map[string]string{}
		newLabels[clusterv1.ClusterLabelName] = cluster.Name
		newLabels[HelmChartProxyLabelName] = helmChartProxy.Name
		helmReleaseProxy.Labels = newLabels

		helmReleaseProxy.Spec.ClusterRef = &corev1.ObjectReference{
			Kind:       cluster.Kind,
			APIVersion: cluster.APIVersion,
			Name:       cluster.Name,
			Namespace:  cluster.Namespace,
		}
	} else {
		changed := false
		if existing.Spec.ChartName != helmChartProxy.Spec.ChartName {
			changed = true
		}
		if existing.Spec.RepoURL != helmChartProxy.Spec.RepoURL {
			changed = true
		}
		if existing.Spec.ReleaseName != helmChartProxy.Spec.ReleaseName {
			changed = true
		}
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
	helmReleaseProxy.Spec.Values = parsedValues

	return helmReleaseProxy
}
