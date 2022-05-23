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
	"fmt"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
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
const selectorKey = "helmChartProxySelector"

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

	labelSelector := helmChartProxy.Spec.ClusterSelector
	log.V(2).Info("HelmChartProxy labels are", "labels", labelSelector)

	log.V(2).Info("Getting list of clusters with labels")
	clusterList, err := r.listClustersWithLabels(ctx, labelSelector)
	if err != nil {
		helmChartProxy.Status.FailureReason = to.StringPtr((errors.Wrapf(err, "failed to list clusters with label selector %+v", labelSelector.MatchLabels).Error()))
		helmChartProxy.Status.Ready = false

		return ctrl.Result{}, err
	}
	if clusterList == nil {
		log.V(2).Info("No clusters found")
	}
	for _, cluster := range clusterList.Items {
		log.V(2).Info("Found cluster", "name", cluster.Name)
	}

	labels := map[string]string{
		selectorKey: helmChartProxy.Name,
	}
	releaseList, err := r.listInstalledReleases(ctx, labels)
	if releaseList == nil {
		log.V(2).Info("No releases found")
	}
	for _, release := range releaseList.Items {
		log.V(2).Info("Found release", "name", release.Name)
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
			if err := r.reconcileDeleteChart(ctx, helmChartProxy, releaseList.Items); err != nil {
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
func (r *HelmChartProxyReconciler) reconcileNormal(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, clusters []clusterv1.Cluster) error {
	log := ctrl.LoggerFrom(ctx)

	if len(clusters) == 0 {
		log.V(2).Info("No clusters to reconcile")

		return nil
	}

	for _, cluster := range clusters {
		kubeconfigPath, err := internal.WriteClusterKubeconfigToFile(ctx, &cluster)
		if err != nil {
			log.Error(err, "failed to get kubeconfig for cluster", "cluster", cluster.Name)
			return err
		}

		err = r.reconcileCluster(ctx, helmChartProxy, &cluster, kubeconfigPath)
		if err != nil {
			log.Error(err, "failed to reconcile chart on cluster", "cluster", cluster.Name)
			return errors.Wrapf(err, "failed to reconcile HelmChartProxy %s on cluster %s", helmChartProxy.Name, cluster.Name)
		}
	}

	return nil
}

func (r *HelmChartProxyReconciler) reconcileCluster(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, cluster *clusterv1.Cluster, kubeconfigPath string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Reconciling HelmChartProxy on cluster", "HelmChartProxy", helmChartProxy.Name, "cluster", cluster.Name)

	releases, err := internal.ListHelmReleases(ctx, kubeconfigPath)
	if err != nil {
		log.Error(err, "failed to list releases")
	}
	log.V(2).Info("Querying existing releases:")
	for _, release := range releases {
		log.V(2).Info("Release found on cluster", "releaseName", release.Name, "cluster", cluster.Name, "revision", release.Version)
	}

	values, err := internal.ParseValues(ctx, r.Client, kubeconfigPath, helmChartProxy.Spec, cluster)
	if err != nil {
		return errors.Wrapf(err, "failed to parse values on cluster %s", cluster.Name)
	}

	existing, err := internal.GetHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error getting release from cluster", "cluster", cluster.Name)

		if err.Error() == "release: not found" {
			// Go ahead and create chart
			release, err := internal.InstallHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec, values)
			if err != nil {
				log.V(2).Error(err, "error installing chart with Helm on cluster", "cluster", cluster.Name)
				return errors.Wrapf(err, "failed to install chart on cluster %s", cluster.Name)
			}
			if release != nil {
				log.V(2).Info((fmt.Sprintf("Release '%s' successfully installed on cluster %s, revision = %d", release.Name, cluster.Name, release.Version)))
				// addClusterRefToStatusList(ctx, helmChartProxy, cluster)
			}

			return nil
		}

		return err
	}

	if existing != nil {
		// TODO: add logic for updating an existing release
		log.V(2).Info(fmt.Sprintf("Release '%s' already installed on cluster %s, running upgrade", existing.Name, cluster.Name))
		release, upgraded, err := internal.UpgradeHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec, values)
		if err != nil {
			log.V(2).Error(err, "error upgrading chart with Helm on cluster", "cluster", cluster.Name)
			return errors.Wrapf(err, "error upgrading chart with Helm on cluster %s", cluster.Name)
		}
		if release != nil && upgraded {
			log.V(2).Info((fmt.Sprintf("Release '%s' successfully upgraded on cluster %s, revision = %d", release.Name, cluster.Name, release.Version)))
			// addClusterRefToStatusList(ctx, helmChartProxy, cluster)
		}
	}

	return nil
}

// reconcileDeleteChart...
func (r *HelmChartProxyReconciler) reconcileDeleteChart(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, releases []addonsv1beta1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	for _, release := range releases {
		kubeconfigPath, err := internal.WriteClusterKubeconfigToFile(ctx, &cluster)
		if err != nil {
			log.Error(err, "failed to get kubeconfig for cluster", "cluster", cluster.Name)
			return errors.Wrapf(err, "failed to get kubeconfig for cluster %s", cluster.Name)
		}
		err = r.reconcileDeleteCluster(ctx, helmChartProxy, &cluster, kubeconfigPath)
		if err != nil {
			log.Error(err, "failed to delete chart on cluster", "cluster", cluster.Name)
			return errors.Wrapf(err, "failed to delete HelmChartProxy %s on cluster %s", helmChartProxy.Name, cluster.Name)
		}
	}

	return nil
}

func (r *HelmChartProxyReconciler) reconcileDeleteCluster(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, cluster *clusterv1.Cluster, kubeconfigPath string) error {
	log := ctrl.LoggerFrom(ctx)

	_, err := internal.GetHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error getting release from cluster", "cluster", cluster.Name)

		if err.Error() == "release: not found" {
			log.V(2).Info(fmt.Sprintf("Release '%s' not found on cluster %s, nothing to do for uninstall", helmChartProxy.Spec.ReleaseName, cluster.Name))
			return nil
		}

		return err
	}

	log.V(2).Info("Preparing to uninstall release on cluster", "releaseName", helmChartProxy.Spec.ReleaseName, "clusterName", cluster.Name)

	response, err := internal.UninstallHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec)
	if err != nil {
		log.V(2).Info("Error uninstalling chart with Helm:", err)
		return errors.Wrapf(err, "error uninstalling chart with Helm on cluster %s", cluster.Name)
	}

	log.V(2).Info((fmt.Sprintf("Chart '%s' successfully uninstalled on cluster %s", helmChartProxy.Spec.ChartName, cluster.Name)))

	if response != nil && response.Info != "" {
		log.V(2).Info(fmt.Sprintf("Response is %s", response.Info))
	}

	return nil
}

func (r *HelmChartProxyReconciler) listClustersWithLabels(ctx context.Context, labelSelector metav1.LabelSelector) (*clusterv1.ClusterList, error) {
	clusterList := &clusterv1.ClusterList{}
	labels := labelSelector.MatchLabels
	// Empty labels should match nothing, not everything
	if len(labels) == 0 {
		return nil, nil
	}

	// TODO: should we use ctrlClient.MatchingLabels or try to use the labelSelector itself?
	if err := r.Client.List(ctx, clusterList, ctrlClient.MatchingLabels(labels)); err != nil {
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

// func addClusterRefToStatusList(ctx context.Context, proxy *addonsv1beta1.HelmChartProxy, cluster *clusterv1.Cluster) {
// 	if proxy.Status.InstalledClusters == nil {
// 		proxy.Status.InstalledClusters = make([]corev1.ObjectReference, 1)
// 	}

// 	proxy.Status.InstalledClusters = append(proxy.Status.InstalledClusters, corev1.ObjectReference{
// 		Kind:       cluster.Kind,
// 		APIVersion: cluster.APIVersion,
// 		Name:       cluster.Name,
// 		Namespace:  cluster.Namespace,
// 	})
// }

// createOrUpdateHelmReleaseProxy...
func (r *HelmChartProxyReconciler) createOrUpdateHelmReleaseProxy(ctx context.Context, cluster *clusterv1.Cluster, helmChartProxy *addonsv1beta1.HelmChartProxy) (*addonsv1beta1.HelmReleaseProxy, error) {
	helmReleaseProxy := &addonsv1beta1.HelmReleaseProxy{}
	helmReleaseProxyKey := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}

	if err := r.Client.Get(ctx, helmReleaseProxyKey, helmReleaseProxy); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		helmReleaseProxy.Name = cluster.Name
		helmReleaseProxy.Namespace = cluster.Namespace
		helmReleaseProxy.OwnerReferences = util.EnsureOwnerRef(helmReleaseProxy.OwnerReferences, metav1.OwnerReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       cluster.Name,
			UID:        cluster.UID,
		})
		helmReleaseProxy.OwnerReferences = util.EnsureOwnerRef(helmReleaseProxy.OwnerReferences, *metav1.NewControllerRef(helmChartProxy, helmChartProxy.GroupVersionKind()))

		helmReleaseProxy.Spec.ClusterName = cluster.Name
		if err := r.Client.Create(ctx, helmReleaseProxy); err != nil {
			if apierrors.IsAlreadyExists(err) {
				if err = r.Client.Get(ctx, helmReleaseProxyKey, helmReleaseProxy); err != nil {
					return nil, err
				}
				return helmReleaseProxy, nil
			}
			return nil, errors.Wrapf(err, "failed to create helmReleaseProxy for cluster: %s/%s", cluster.Namespace, cluster.Name)
		}
	}

	return helmReleaseProxy, nil
}

// deleteHelmReleaseProxy...
func (r *HelmChartProxyReconciler) deleteHelmReleaseProxy(ctx context.Context, helmReleaseProxy *addonsv1beta1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	if err := r.Client.Delete(ctx, helmReleaseProxy); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("HelmReleaseProxy already deleted, nothing to do", "helmReleaseProxy", helmReleaseProxy.Name)
			return nil
		}
		return errors.Wrapf(err, "failed to create helmReleaseProxy: %s/%s", helmReleaseProxy.Name)
	}

	return nil
}
