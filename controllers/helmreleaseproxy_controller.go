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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"
	"cluster-api-addon-helm/internal"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/cluster"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
)

// HelmReleaseProxyReconciler reconciles a HelmReleaseProxy object
type HelmReleaseProxyReconciler struct {
	ctrlClient.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HelmReleaseProxy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *HelmReleaseProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Beginning reconcilation for HelmReleaseProxy", "requestNamespace", req.Namespace, "requestName", req.Name)

	// Fetch the HelmReleaseProxy instance.
	helmReleaseProxy := &addonsv1beta1.HelmReleaseProxy{}
	if err := r.Client.Get(ctx, req.NamespacedName, helmReleaseProxy); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("HelmReleaseProxy resource not found, skipping reconciliation", "helmReleaseProxy", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// TODO: should patch helper return an error when the object has been deleted?
	patchHelper, err := patch.NewHelper(helmReleaseProxy, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to init patch helper")
	}

	defer func() {
		log.V(2).Info("Preparing to patch HelmReleaseProxy", "helmReleaseProxy", helmReleaseProxy.Name)
		if err := patchHelper.Patch(ctx, helmReleaseProxy); err != nil && reterr == nil {
			reterr = err
			log.Error(err, "failed to patch HelmReleaseProxy", "helmReleaseProxy", helmReleaseProxy.Name)
			return
		}
		log.V(2).Info("Successfully patched HelmReleaseProxy", "helmReleaseProxy", helmReleaseProxy.Name)
	}()

	labelSelector := helmReleaseProxy.Spec.ClusterSelector
	log.V(2).Info("HelmReleaseProxy labels are", "labels", labelSelector)

	log.V(2).Info("Getting list of clusters with labels")
	clusterList, err := r.listClustersWithLabels(ctx, labelSelector)
	if err != nil {
		helmReleaseProxy.Status.FailureReason = to.StringPtr((errors.Wrapf(err, "failed to list clusters with label selector %+v", labelSelector.MatchLabels).Error()))
		helmReleaseProxy.Status.Ready = false

		return ctrl.Result{}, err
	}
	if clusterList == nil {
		log.V(2).Info("No clusters found")
	}
	for _, cluster := range clusterList.Items {
		log.V(2).Info("Found cluster", "name", cluster.Name)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if helmReleaseProxy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(helmReleaseProxy, finalizer) {
			controllerutil.AddFinalizer(helmReleaseProxy, finalizer)
			if err := r.Update(ctx, helmReleaseProxy); err != nil {
				helmReleaseProxy.Status.FailureReason = to.StringPtr(errors.Wrapf(err, "failed to add finalizer").Error())
				helmReleaseProxy.Status.Ready = false
				if err := r.Status().Update(ctx, helmReleaseProxy); err != nil {
					log.Error(err, "unable to update HelmReleaseProxy status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(helmReleaseProxy, finalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.reconcileDelete(ctx, helmReleaseProxy, clusterList.Items); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				helmReleaseProxy.Status.FailureReason = to.StringPtr(err.Error())
				helmReleaseProxy.Status.Ready = false
				if err := r.Status().Update(ctx, helmReleaseProxy); err != nil {
					log.Error(err, "unable to update HelmReleaseProxy status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}

			helmReleaseProxy.Status.Ready = true
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(helmReleaseProxy, finalizer)
			if err := r.Update(ctx, helmReleaseProxy); err != nil {
				helmReleaseProxy.Status.FailureReason = to.StringPtr(errors.Wrapf(err, "failed to remove finalizer").Error())
				helmReleaseProxy.Status.Ready = false
				if err := r.Status().Update(ctx, helmReleaseProxy); err != nil {
					log.Error(err, "unable to update HelmReleaseProxy status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	log.V(2).Info("Reconciling HelmReleaseProxy", "randomName", helmReleaseProxy.Name)
	err = r.reconcileNormal(ctx, helmReleaseProxy, clusterList.Items)
	if err != nil {
		helmReleaseProxy.Status.Ready = false

		return ctrl.Result{}, err
	}

	helmReleaseProxy.Status.FailureReason = nil
	helmReleaseProxy.Status.Ready = true
	if err := r.Status().Update(ctx, helmReleaseProxy); err != nil {
		log.Error(err, "unable to update HelmReleaseProxy status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmReleaseProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1beta1.HelmReleaseProxy{}).
		// Watches(
		// 	&source.Kind{Type: &v1beta1.HelmReleaseProxy{}},
		// 	handler.EnqueueRequestsFromMapFunc(r.findProxyForSecret),
		// 	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		// ).
		Complete(r)
}

// reconcileNormal,...
func (r *HelmReleaseProxyReconciler) reconcileNormal(ctx context.Context, helmReleaseProxy *addonsv1beta1.HelmReleaseProxy, cluster *clusterv1.Cluster, kubeconfigPath string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Reconciling HelmReleaseProxy on cluster", "HelmReleaseProxy", helmReleaseProxy.Name, "cluster", cluster.Name)

	releases, err := internal.ListHelmReleases(ctx, kubeconfigPath)
	if err != nil {
		log.Error(err, "failed to list releases")
	}
	log.V(2).Info("Querying existing releases:")
	for _, release := range releases {
		log.V(2).Info("Release found on cluster", "releaseName", release.Name, "cluster", cluster.Name, "revision", release.Version)
	}

	values, err := internal.ParseValues(ctx, r.Client, kubeconfigPath, helmReleaseProxy.Spec, cluster)
	if err != nil {
		return errors.Wrapf(err, "failed to parse values on cluster %s", cluster.Name)
	}

	existing, err := internal.GetHelmRelease(ctx, kubeconfigPath, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error getting release from cluster", "cluster", cluster.Name)

		if err.Error() == "release: not found" {
			// Go ahead and create chart
			release, err := internal.InstallHelmRelease(ctx, kubeconfigPath, helmReleaseProxy.Spec, values)
			if err != nil {
				log.V(2).Error(err, "error installing chart with Helm on cluster", "cluster", cluster.Name)
				return errors.Wrapf(err, "failed to install chart on cluster %s", cluster.Name)
			}
			if release != nil {
				log.V(2).Info((fmt.Sprintf("Release '%s' successfully installed on cluster %s, revision = %d", release.Name, cluster.Name, release.Version)))
				// addClusterRefToStatusList(ctx, helmReleaseProxy, cluster)
			}

			return nil
		}

		return err
	}

	if existing != nil {
		// TODO: add logic for updating an existing release
		log.V(2).Info(fmt.Sprintf("Release '%s' already installed on cluster %s, running upgrade", existing.Name, cluster.Name))
		release, upgraded, err := internal.UpgradeHelmRelease(ctx, kubeconfigPath, helmReleaseProxy.Spec, values)
		if err != nil {
			log.V(2).Error(err, "error upgrading chart with Helm on cluster", "cluster", cluster.Name)
			return errors.Wrapf(err, "error upgrading chart with Helm on cluster %s", cluster.Name)
		}
		if release != nil && upgraded {
			log.V(2).Info((fmt.Sprintf("Release '%s' successfully upgraded on cluster %s, revision = %d", release.Name, cluster.Name, release.Version)))
			// addClusterRefToStatusList(ctx, helmReleaseProxy, cluster)
		}
	}

	return nil
}

// reconcileDelete...
func (r *HelmReleaseProxyReconciler) reconcileDelete(ctx context.Context, helmReleaseProxy *addonsv1beta1.HelmReleaseProxy, cluster *clusterv1.Cluster, kubeconfigPath string) error {
	log := ctrl.LoggerFrom(ctx)

	_, err := internal.GetHelmRelease(ctx, kubeconfigPath, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error getting release from cluster", "cluster", cluster.Name)

		if err.Error() == "release: not found" {
			log.V(2).Info(fmt.Sprintf("Release '%s' not found on cluster %s, nothing to do for uninstall", helmReleaseProxy.Spec.ReleaseName, cluster.Name))
			return nil
		}

		return err
	}

	log.V(2).Info("Preparing to uninstall release on cluster", "releaseName", helmReleaseProxy.Spec.ReleaseName, "clusterName", cluster.Name)

	response, err := internal.UninstallHelmRelease(ctx, kubeconfigPath, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Info("Error uninstalling chart with Helm:", err)
		return errors.Wrapf(err, "error uninstalling chart with Helm on cluster %s", cluster.Name)
	}

	log.V(2).Info((fmt.Sprintf("Chart '%s' successfully uninstalled on cluster %s", helmReleaseProxy.Spec.ChartName, cluster.Name)))

	if response != nil && response.Info != "" {
		log.V(2).Info(fmt.Sprintf("Response is %s", response.Info))
	}

	return nil
}
