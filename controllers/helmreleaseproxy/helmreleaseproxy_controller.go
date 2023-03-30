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

package helmreleaseproxy

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	helmDriver "helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"sigs.k8s.io/cluster-api-addon-provider-helm/internal"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
)

// HelmReleaseProxyReconciler reconciles a HelmReleaseProxy object
type HelmReleaseProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmReleaseProxyReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	_ = ctrl.LoggerFrom(ctx)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&addonsv1alpha1.HelmReleaseProxy{}).
		// Watches(
		// 	&source.Kind{Type: &v1alpha1.HelmReleaseProxy{}},
		// 	handler.EnqueueRequestsFromMapFunc(r.findProxyForSecret),
		// 	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		// ).
		Complete(r)
}

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;watch
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;watch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io;clusterctl.cluster.x-k8s.io,resources=*,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HelmReleaseProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Beginning reconcilation for HelmReleaseProxy", "requestNamespace", req.Namespace, "requestName", req.Name)

	// Fetch the HelmReleaseProxy instance.
	helmReleaseProxy := &addonsv1alpha1.HelmReleaseProxy{}
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

	initalizeConditions(ctx, patchHelper, helmReleaseProxy)

	defer func() {
		log.V(2).Info("Preparing to patch HelmReleaseProxy", "helmReleaseProxy", helmReleaseProxy.Name)
		log.V(2).Info("HelmReleaseProxy return error is", "reterr", reterr)
		if err := patchHelmReleaseProxy(ctx, patchHelper, helmReleaseProxy); err != nil && reterr == nil {
			reterr = err
			log.Error(err, "failed to patch HelmReleaseProxy", "helmReleaseProxy", helmReleaseProxy.Name)
			return
		}
		log.V(2).Info("Successfully patched HelmReleaseProxy", "helmReleaseProxy", helmReleaseProxy.Name)
	}()

	cluster := &clusterv1.Cluster{}
	clusterKey := client.ObjectKey{
		Namespace: helmReleaseProxy.Spec.ClusterRef.Namespace,
		Name:      helmReleaseProxy.Spec.ClusterRef.Name,
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if helmReleaseProxy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(helmReleaseProxy, addonsv1alpha1.HelmReleaseProxyFinalizer) {
			controllerutil.AddFinalizer(helmReleaseProxy, addonsv1alpha1.HelmReleaseProxyFinalizer)
			if err := patchHelmReleaseProxy(ctx, patchHelper, helmReleaseProxy); err != nil {
				// TODO: Should we try to set the error here? If we can't remove the finalizer we likely can't update the status either.
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(helmReleaseProxy, addonsv1alpha1.HelmReleaseProxyFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.Client.Get(ctx, clusterKey, cluster); err == nil {
				log.V(2).Info("Getting kubeconfig for cluster", "cluster", cluster.Name)
				kubeconfig, err := internal.GetClusterKubeconfig(ctx, cluster)
				if err != nil {
					wrappedErr := errors.Wrapf(err, "failed to get kubeconfig for cluster")
					conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition, addonsv1alpha1.GetKubeconfigFailedReason, clusterv1.ConditionSeverityError, wrappedErr.Error())

					return ctrl.Result{}, wrappedErr
				}
				conditions.MarkTrue(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition)

				if err := r.reconcileDelete(ctx, helmReleaseProxy, kubeconfig); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
					return ctrl.Result{}, err
				}
			} else if apierrors.IsNotFound(err) {
				// Cluster is gone, so we should remove our finalizer from the list and delete
				log.V(2).Info("Cluster not found, no need to delete external dependency", "cluster", cluster.Name)
				// TODO: should we set a condition here?
			} else {
				wrappedErr := errors.Wrapf(err, "failed to get cluster %s/%s", clusterKey.Namespace, clusterKey.Name)
				conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition, addonsv1alpha1.GetClusterFailedReason, clusterv1.ConditionSeverityError, wrappedErr.Error())

				return ctrl.Result{}, wrappedErr
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(helmReleaseProxy, addonsv1alpha1.HelmReleaseProxyFinalizer)
			if err := patchHelmReleaseProxy(ctx, patchHelper, helmReleaseProxy); err != nil {
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
		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition, addonsv1alpha1.GetClusterFailedReason, clusterv1.ConditionSeverityError, wrappedErr.Error())

		return ctrl.Result{}, wrappedErr
	}

	log.V(2).Info("Getting kubeconfig for cluster", "cluster", cluster.Name)
	kubeconfig, err := internal.GetClusterKubeconfig(ctx, cluster)
	if err != nil {
		wrappedErr := errors.Wrapf(err, "failed to get kubeconfig for cluster")
		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition, addonsv1alpha1.GetKubeconfigFailedReason, clusterv1.ConditionSeverityError, wrappedErr.Error())

		return ctrl.Result{}, wrappedErr
	}
	conditions.MarkTrue(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition)

	log.V(2).Info("Reconciling HelmReleaseProxy", "releaseProxyName", helmReleaseProxy.Name)
	err = r.reconcileNormal(ctx, helmReleaseProxy, kubeconfig)

	return ctrl.Result{}, err
}

// reconcileNormal handles HelmReleaseProxy reconciliation when it is not being deleted. This will install or upgrade the HelmReleaseProxy on the Cluster.
// It will set the ReleaseName on the HelmReleaseProxy if the name is generated and also set the release status and release revision.
func (r *HelmReleaseProxyReconciler) reconcileNormal(ctx context.Context, helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy, kubeconfig string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Reconciling HelmReleaseProxy on cluster", "HelmReleaseProxy", helmReleaseProxy.Name, "cluster", helmReleaseProxy.Spec.ClusterRef.Name)

	// TODO: add this here or in HelmChartProxy controller?
	if helmReleaseProxy.Spec.ReleaseName == "" {
		helmReleaseProxy.ObjectMeta.SetAnnotations(map[string]string{
			addonsv1alpha1.IsReleaseNameGeneratedAnnotation: "true",
		})
	}

	log.V(2).Info(fmt.Sprintf("Preparing to install or upgrade release '%s' on cluster %s", helmReleaseProxy.Spec.ReleaseName, helmReleaseProxy.Spec.ClusterRef.Name))
	release, changed, err := internal.InstallOrUpgradeHelmRelease(ctx, kubeconfig, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error installing or updating chart with Helm on cluster", "cluster", helmReleaseProxy.Spec.ClusterRef.Name)
		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmInstallOrUpgradeFailedReason, clusterv1.ConditionSeverityError, err.Error())

		return errors.Wrapf(err, "error installing or updating chart with Helm on cluster %s", helmReleaseProxy.Spec.ClusterRef.Name)
	}
	if release != nil {
		if changed {
			log.V(2).Info((fmt.Sprintf("Release '%s' successfully installed or updated on cluster %s, revision = %d", release.Name, helmReleaseProxy.Spec.ClusterRef.Name, release.Version)))
		} else {
			log.V(2).Info((fmt.Sprintf("Release '%s' is up to date on cluster %s, no upgrade required, revision = %d", release.Name, helmReleaseProxy.Spec.ClusterRef.Name, release.Version)))
		}

		helmReleaseProxy.SetReleaseStatus(release.Info.Status.String())
		helmReleaseProxy.SetReleaseRevision(release.Version)
		helmReleaseProxy.SetReleaseName(release.Name)
		conditions.MarkTrue(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition)
	}

	return nil
}

// reconcileDelete handles HelmReleaseProxy deletion. This will uninstall the HelmReleaseProxy on the Cluster or return nil if the HelmReleaseProxy is not found.
func (r *HelmReleaseProxyReconciler) reconcileDelete(ctx context.Context, helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy, kubeconfig string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Deleting HelmReleaseProxy on cluster", "HelmReleaseProxy", helmReleaseProxy.Name, "cluster", helmReleaseProxy.Spec.ClusterRef.Name)

	_, err := internal.GetHelmRelease(ctx, kubeconfig, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error getting release from cluster", "cluster", helmReleaseProxy.Spec.ClusterRef.Name)

		if err == helmDriver.ErrReleaseNotFound {
			log.V(2).Info(fmt.Sprintf("Release '%s' not found on cluster %s, nothing to do for uninstall", helmReleaseProxy.Spec.ReleaseName, helmReleaseProxy.Spec.ClusterRef.Name))
			conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmReleaseDeletedReason, clusterv1.ConditionSeverityInfo, "")

			return nil
		}

		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmReleaseGetFailedReason, clusterv1.ConditionSeverityError, err.Error())

		return err
	}

	log.V(2).Info("Preparing to uninstall release on cluster", "releaseName", helmReleaseProxy.Spec.ReleaseName, "clusterName", helmReleaseProxy.Spec.ClusterRef.Name)

	response, err := internal.UninstallHelmRelease(ctx, kubeconfig, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Info("Error uninstalling chart with Helm:", err)
		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmReleaseDeletionFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return errors.Wrapf(err, "error uninstalling chart with Helm on cluster %s", helmReleaseProxy.Spec.ClusterRef.Name)
	}

	log.V(2).Info((fmt.Sprintf("Chart '%s' successfully uninstalled on cluster %s", helmReleaseProxy.Spec.ChartName, helmReleaseProxy.Spec.ClusterRef.Name)))
	conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmReleaseDeletedReason, clusterv1.ConditionSeverityInfo, "")
	if response != nil && response.Info != "" {
		log.V(2).Info(fmt.Sprintf("Response is %s", response.Info))
	}

	return nil
}

func initalizeConditions(ctx context.Context, patchHelper *patch.Helper, helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy) {
	log := ctrl.LoggerFrom(ctx)
	if len(helmReleaseProxy.GetConditions()) == 0 {
		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.PreparingToHelmInstallReason, clusterv1.ConditionSeverityInfo, "Preparing to to install Helm chart")
		if err := patchHelmReleaseProxy(ctx, patchHelper, helmReleaseProxy); err != nil {
			log.Error(err, "failed to patch HelmReleaseProxy with initial conditions")
		}
	}
}

// patchHelmReleaseProxy patches the HelmReleaseProxy object and sets the ReadyCondition as an aggregate of the other condition set.
// TODO: Is this preferrable to client.Update() calls? Based on testing it seems like it avoids race conditions.
func patchHelmReleaseProxy(ctx context.Context, patchHelper *patch.Helper, helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy) error {
	conditions.SetSummary(helmReleaseProxy,
		conditions.WithConditions(
			addonsv1alpha1.HelmReleaseReadyCondition,
			addonsv1alpha1.ClusterAvailableCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		helmReleaseProxy,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			addonsv1alpha1.HelmReleaseReadyCondition,
			addonsv1alpha1.ClusterAvailableCondition,
		}},
		patch.WithStatusObservedGeneration{},
	)
}
