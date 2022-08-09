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

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/release"
	helmDriver "helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1beta1 "cluster-api-addon-provider-helm/api/v1beta1"
	"cluster-api-addon-provider-helm/internal"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
)

// HelmReleaseProxyReconciler reconciles a HelmReleaseProxy object
type HelmReleaseProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;watch
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;watch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

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
		log.V(2).Info("HelmReleaseProxy return error is", "reterr", reterr)
		if err := patchHelper.Patch(ctx, helmReleaseProxy); err != nil && reterr == nil {
			reterr = err
			log.Error(err, "failed to patch HelmReleaseProxy", "helmReleaseProxy", helmReleaseProxy.Name)
			return
		}
		log.V(2).Info("Successfully patched HelmReleaseProxy", "helmReleaseProxy", helmReleaseProxy.Name)
	}()

	cluster := &clusterv1.Cluster{}
	clusterKey := ctrlClient.ObjectKey{
		Namespace: helmReleaseProxy.Spec.ClusterRef.Namespace,
		Name:      helmReleaseProxy.Spec.ClusterRef.Name,
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if helmReleaseProxy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(helmReleaseProxy, addonsv1beta1.HelmReleaseProxyFinalizer) {
			controllerutil.AddFinalizer(helmReleaseProxy, addonsv1beta1.HelmReleaseProxyFinalizer)
			if err := r.Update(ctx, helmReleaseProxy); err != nil {
				// TODO: Should we try to set the error here? If we can't remove the finalizer we likely can't update the status either.
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(helmReleaseProxy, addonsv1beta1.HelmReleaseProxyFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.Client.Get(ctx, clusterKey, cluster); err == nil {
				log.V(2).Info("Getting kubeconfig for cluster", "cluster", cluster.Name)
				kubeconfig, err := internal.GetClusterKubeconfig(ctx, cluster)
				if err != nil {
					wrappedErr := errors.Wrapf(err, "failed to get kubeconfig for cluster")
					setReleaseError(helmReleaseProxy, wrappedErr)
					return ctrl.Result{}, wrappedErr
				}

				if err := r.reconcileDelete(ctx, helmReleaseProxy, kubeconfig); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
					setReleaseError(helmReleaseProxy, err)
					return ctrl.Result{}, err
				}
			} else if apierrors.IsNotFound(err) {
				// Cluster is gone, so we should remove our finalizer from the list and delete
				log.V(2).Info("Cluster not found, no need to delete external dependency", "cluster", cluster.Name)
			} else {
				wrappedErr := errors.Wrapf(err, "failed to get cluster %s/%s", clusterKey.Namespace, clusterKey.Name)
				setReleaseError(helmReleaseProxy, wrappedErr)
				return ctrl.Result{}, wrappedErr
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(helmReleaseProxy, addonsv1beta1.HelmReleaseProxyFinalizer)
			if err := r.Update(ctx, helmReleaseProxy); err != nil {
				// TODO: Should we try to set the error here? If we can't remove the finalizer we likely can't update the status either.
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if err := r.Client.Get(ctx, clusterKey, cluster); err != nil {
		// TODO: add check to tell if Cluster is deleted so we can remove the HelmReleaseProxy.
		wrappedErr := errors.Wrapf(err, "failed to get cluster %s/%s", clusterKey.Namespace, clusterKey.Name)
		setReleaseError(helmReleaseProxy, wrappedErr)
		return ctrl.Result{}, wrappedErr
	}

	log.V(2).Info("Getting kubeconfig for cluster", "cluster", cluster.Name)
	kubeconfig, err := internal.GetClusterKubeconfig(ctx, cluster)
	if err != nil {
		wrappedErr := errors.Wrapf(err, "failed to get kubeconfig for cluster")
		setReleaseError(helmReleaseProxy, wrappedErr)
		return ctrl.Result{}, wrappedErr
	}

	log.V(2).Info("Reconciling HelmReleaseProxy", "releaseProxyName", helmReleaseProxy.Name)
	err = r.reconcileNormal(ctx, helmReleaseProxy, kubeconfig)
	setReleaseError(helmReleaseProxy, err)

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmReleaseProxyReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&addonsv1beta1.HelmReleaseProxy{}).
		// Watches(
		// 	&source.Kind{Type: &v1beta1.HelmReleaseProxy{}},
		// 	handler.EnqueueRequestsFromMapFunc(r.findProxyForSecret),
		// 	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		// ).
		Complete(r)
}

// reconcileNormal,...
func (r *HelmReleaseProxyReconciler) reconcileNormal(ctx context.Context, helmReleaseProxy *addonsv1beta1.HelmReleaseProxy, kubeconfig string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Reconciling HelmReleaseProxy on cluster", "HelmReleaseProxy", helmReleaseProxy.Name, "cluster", helmReleaseProxy.Spec.ClusterRef.Name)

	// TODO: add this here or in HelmChartProxy controller?
	if helmReleaseProxy.Spec.ReleaseName == "" {
		helmReleaseProxy.ObjectMeta.SetAnnotations(map[string]string{
			addonsv1beta1.IsReleaseNameGeneratedAnnotation: "true",
		})
	}

	log.V(2).Info(fmt.Sprintf("Preparing to install or upgrade release '%s' on cluster %s", helmReleaseProxy.Spec.ReleaseName, helmReleaseProxy.Spec.ClusterRef.Name))
	release, changed, err := internal.InstallOrUpgradeHelmRelease(ctx, kubeconfig, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error installing or updating chart with Helm on cluster", "cluster", helmReleaseProxy.Spec.ClusterRef.Name)
		return errors.Wrapf(err, "error installing or updating chart with Helm on cluster %s", helmReleaseProxy.Spec.ClusterRef.Name)
	}
	if release != nil {
		if changed {
			log.V(2).Info((fmt.Sprintf("Release '%s' successfully installed or updated on cluster %s, revision = %d", release.Name, helmReleaseProxy.Spec.ClusterRef.Name, release.Version)))
		} else {
			log.V(2).Info((fmt.Sprintf("Release '%s' is up to date on cluster %s, no upgrade required, revision = %d", release.Name, helmReleaseProxy.Spec.ClusterRef.Name, release.Version)))
		}

		setReleaseStatusFields(helmReleaseProxy, release)
		updateReleaseName(helmReleaseProxy, release)
		// addClusterRefToStatusList(ctx, helmReleaseProxy, cluster)
	}

	return nil
}

// reconcileDelete...
func (r *HelmReleaseProxyReconciler) reconcileDelete(ctx context.Context, helmReleaseProxy *addonsv1beta1.HelmReleaseProxy, kubeconfig string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Deleting HelmReleaseProxy on cluster", "HelmReleaseProxy", helmReleaseProxy.Name, "cluster", helmReleaseProxy.Spec.ClusterRef.Name)

	_, err := internal.GetHelmRelease(ctx, kubeconfig, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error getting release from cluster", "cluster", helmReleaseProxy.Spec.ClusterRef.Name)

		if err == helmDriver.ErrReleaseNotFound {
			log.V(2).Info(fmt.Sprintf("Release '%s' not found on cluster %s, nothing to do for uninstall", helmReleaseProxy.Spec.ReleaseName, helmReleaseProxy.Spec.ClusterRef.Name))
			return nil
		}

		return err
	}

	log.V(2).Info("Preparing to uninstall release on cluster", "releaseName", helmReleaseProxy.Spec.ReleaseName, "clusterName", helmReleaseProxy.Spec.ClusterRef.Name)

	response, err := internal.UninstallHelmRelease(ctx, kubeconfig, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Info("Error uninstalling chart with Helm:", err)
		return errors.Wrapf(err, "error uninstalling chart with Helm on cluster %s", helmReleaseProxy.Spec.ClusterRef.Name)
	}

	log.V(2).Info((fmt.Sprintf("Chart '%s' successfully uninstalled on cluster %s", helmReleaseProxy.Spec.ChartName, helmReleaseProxy.Spec.ClusterRef.Name)))

	if response != nil && response.Info != "" {
		log.V(2).Info(fmt.Sprintf("Response is %s", response.Info))
	}

	return nil
}

func setReleaseError(helmReleaseProxy *addonsv1beta1.HelmReleaseProxy, err error) {
	if err != nil {
		helmReleaseProxy.Status.FailureReason = err.Error()
		helmReleaseProxy.Status.Ready = false
	} else {
		helmReleaseProxy.Status.FailureReason = ""
		helmReleaseProxy.Status.Ready = true
	}
}

func setReleaseStatusFields(helmReleaseProxy *addonsv1beta1.HelmReleaseProxy, release *release.Release) {
	helmReleaseProxy.Status.Status = release.Info.Status.String() // See pkg/release/status.go in Helm for possible values
	helmReleaseProxy.Status.Revision = release.Version
	helmReleaseProxy.Status.Namespace = release.Namespace // TODO: Add a way to configure the namespace
}

func updateReleaseName(helmReleaseProxy *addonsv1beta1.HelmReleaseProxy, release *release.Release) {
	if helmReleaseProxy.Spec.ReleaseName == "" {
		helmReleaseProxy.Spec.ReleaseName = release.Name
	}
}
