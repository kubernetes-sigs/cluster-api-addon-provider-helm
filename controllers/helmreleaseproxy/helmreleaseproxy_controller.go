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
	"os"

	"github.com/pkg/errors"
	helmRelease "helm.sh/helm/v3/pkg/release"
	helmDriver "helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"sigs.k8s.io/cluster-api-addon-provider-helm/internal"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// HelmReleaseProxyReconciler reconciles a HelmReleaseProxy object.
type HelmReleaseProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmReleaseProxyReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)

	clusterToHelmReleaseProxies, err := util.ClusterToTypedObjectsMapper(mgr.GetClient(), &addonsv1alpha1.HelmReleaseProxyList{}, mgr.GetScheme())
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&addonsv1alpha1.HelmReleaseProxy{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToHelmReleaseProxies),
			builder.WithPredicates(
				predicates.All(ctrl.LoggerFrom(ctx),
					predicates.Any(ctrl.LoggerFrom(ctx),
						predicates.ClusterUnpaused(ctrl.LoggerFrom(ctx)),
						predicates.ClusterControlPlaneInitialized(ctrl.LoggerFrom(ctx)),
					),
					predicates.ResourceHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue),
				),
			)).
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

	log.V(2).Info("Beginning reconciliation for HelmReleaseProxy", "requestNamespace", req.Namespace, "requestName", req.Name)

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
		log.V(2).Info("Preparing to patch HelmReleaseProxy with return error", "helmReleaseProxy", helmReleaseProxy.Name, "reterr", reterr)
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

	k := internal.KubeconfigGetter{}
	client := &internal.HelmClient{}

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
				kubeconfig, err := k.GetClusterKubeconfig(ctx, cluster)
				if err != nil {
					wrappedErr := errors.Wrapf(err, "failed to get kubeconfig for cluster")
					conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition, addonsv1alpha1.GetKubeconfigFailedReason, clusterv1.ConditionSeverityError, wrappedErr.Error())

					return ctrl.Result{}, wrappedErr
				}
				conditions.MarkTrue(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition)

				if err := r.reconcileDelete(ctx, helmReleaseProxy, client, kubeconfig); err != nil {
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

	if !conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition) {
		log.Info("Waiting for the control plane to be initialized")
		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition, clusterv1.WaitingForControlPlaneAvailableReason, clusterv1.ConditionSeverityInfo, "")

		// Return since the kubeconfig won't be available or useable until the API server is reachable.
		// The controller watches the Cluster for control plane initialization in SetupWithManager, so a requeue is not necessary.
		return ctrl.Result{}, nil
	}

	log.V(2).Info("Getting kubeconfig for cluster", "cluster", cluster.Name)
	kubeconfig, err := k.GetClusterKubeconfig(ctx, cluster)
	if err != nil {
		wrappedErr := errors.Wrapf(err, "failed to get kubeconfig for cluster")
		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition, addonsv1alpha1.GetKubeconfigFailedReason, clusterv1.ConditionSeverityError, wrappedErr.Error())

		return ctrl.Result{}, wrappedErr
	}
	conditions.MarkTrue(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition)

	credentialsPath, err := r.getCredentials(ctx, helmReleaseProxy)
	if err != nil {
		wrappedErr := errors.Wrapf(err, "failed to get credentials for cluster")
		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.ClusterAvailableCondition, addonsv1alpha1.GetCredentialsFailedReason, clusterv1.ConditionSeverityError, wrappedErr.Error())

		return ctrl.Result{}, wrappedErr
	}

	defer func() {
		if err := os.Remove(credentialsPath); err != nil {
			log.Error(err, "failed to remove credentials file in path", "credentialsPath", credentialsPath)
		}
	}()

	log.V(2).Info("Reconciling HelmReleaseProxy", "releaseProxyName", helmReleaseProxy.Name)
	err = r.reconcileNormal(ctx, helmReleaseProxy, client, credentialsPath, kubeconfig)

	return ctrl.Result{}, err
}

// reconcileNormal handles HelmReleaseProxy reconciliation when it is not being deleted. This will install or upgrade the HelmReleaseProxy on the Cluster.
// It will set the ReleaseName on the HelmReleaseProxy if the name is generated and also set the release status and release revision.
func (r *HelmReleaseProxyReconciler) reconcileNormal(ctx context.Context, helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy, client internal.Client, credentialsPath string, kubeconfig string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Reconciling HelmReleaseProxy on cluster", "HelmReleaseProxy", helmReleaseProxy.Name, "cluster", helmReleaseProxy.Spec.ClusterRef.Name)

	// TODO: add this here or in HelmChartProxy controller?
	if helmReleaseProxy.Spec.ReleaseName == "" {
		helmReleaseProxy.ObjectMeta.SetAnnotations(map[string]string{
			addonsv1alpha1.IsReleaseNameGeneratedAnnotation: "true",
		})
	}

	release, err := client.InstallOrUpgradeHelmRelease(ctx, kubeconfig, credentialsPath, helmReleaseProxy.Spec)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to install or upgrade release '%s' on cluster %s", helmReleaseProxy.Spec.ReleaseName, helmReleaseProxy.Spec.ClusterRef.Name))
		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmInstallOrUpgradeFailedReason, clusterv1.ConditionSeverityError, err.Error())
	}
	if release != nil {
		log.V(2).Info((fmt.Sprintf("Release '%s' exists on cluster %s, revision = %d", release.Name, helmReleaseProxy.Spec.ClusterRef.Name, release.Version)))

		status := release.Info.Status
		helmReleaseProxy.SetReleaseStatus(status.String())
		helmReleaseProxy.SetReleaseRevision(release.Version)
		helmReleaseProxy.SetReleaseName(release.Name)

		switch {
		case status == helmRelease.StatusDeployed:
			conditions.MarkTrue(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition)
		case status.IsPending():
			conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmReleasePendingReason, clusterv1.ConditionSeverityInfo, fmt.Sprintf("Helm release is in a pending state: %s", status))
		case status == helmRelease.StatusFailed && err == nil:
			log.Info("Helm release failed without error, this might be unexpected", "release", release.Name, "cluster", helmReleaseProxy.Spec.ClusterRef.Name)
			conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmInstallOrUpgradeFailedReason, clusterv1.ConditionSeverityError, fmt.Sprintf("Helm release failed: %s", status))
			// TODO: should we set the error state again here?
		}
	}

	return err
}

// reconcileDelete handles HelmReleaseProxy deletion. This will uninstall the HelmReleaseProxy on the Cluster or return nil if the HelmReleaseProxy is not found.
func (r *HelmReleaseProxyReconciler) reconcileDelete(ctx context.Context, helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy, client internal.Client, kubeconfig string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Deleting HelmReleaseProxy on cluster", "HelmReleaseProxy", helmReleaseProxy.Name, "cluster", helmReleaseProxy.Spec.ClusterRef.Name)

	_, err := client.GetHelmRelease(ctx, kubeconfig, helmReleaseProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error getting release from cluster", "cluster", helmReleaseProxy.Spec.ClusterRef.Name)

		if errors.Is(err, helmDriver.ErrReleaseNotFound) {
			log.V(2).Info(fmt.Sprintf("Release '%s' not found on cluster %s, nothing to do for uninstall", helmReleaseProxy.Spec.ReleaseName, helmReleaseProxy.Spec.ClusterRef.Name))
			conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmReleaseDeletedReason, clusterv1.ConditionSeverityInfo, "")

			return nil
		}

		conditions.MarkFalse(helmReleaseProxy, addonsv1alpha1.HelmReleaseReadyCondition, addonsv1alpha1.HelmReleaseGetFailedReason, clusterv1.ConditionSeverityError, err.Error())

		return err
	}

	log.V(2).Info("Preparing to uninstall release on cluster", "releaseName", helmReleaseProxy.Spec.ReleaseName, "clusterName", helmReleaseProxy.Spec.ClusterRef.Name)

	response, err := client.UninstallHelmRelease(ctx, kubeconfig, helmReleaseProxy.Spec)
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
			addonsv1alpha1.ClusterAvailableCondition,
			addonsv1alpha1.HelmReleaseReadyCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		helmReleaseProxy,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			addonsv1alpha1.ClusterAvailableCondition,
			addonsv1alpha1.HelmReleaseReadyCondition,
		}},
		patch.WithStatusObservedGeneration{},
	)
}

// getCredentials fetches the OCI credentials from a Secret and writes them to a temporary file it returns the path to the temporary file.
func (r *HelmReleaseProxyReconciler) getCredentials(ctx context.Context, helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy) (string, error) {
	credentialsPath := ""
	if helmReleaseProxy.Spec.Credentials != nil && helmReleaseProxy.Spec.Credentials.Secret.Name != "" {
		// By default, the secret is in the same namespace as the HelmReleaseProxy
		if helmReleaseProxy.Spec.Credentials.Secret.Namespace == "" {
			helmReleaseProxy.Spec.Credentials.Secret.Namespace = helmReleaseProxy.Namespace
		}
		credentialsValues, err := r.getCredentialsFromSecret(ctx, helmReleaseProxy.Spec.Credentials.Secret.Name, helmReleaseProxy.Spec.Credentials.Secret.Namespace, helmReleaseProxy.Spec.Credentials.Key)
		if err != nil {
			return "", err
		}

		// Write to a file
		filename, err := writeCredentialsToFile(ctx, credentialsValues)
		if err != nil {
			return "", err
		}

		credentialsPath = filename
	}

	return credentialsPath, nil
}

// getCredentialsFromSecret returns the OCI credentials from a Secret.
func (r *HelmReleaseProxyReconciler) getCredentialsFromSecret(ctx context.Context, name, namespace, key string) ([]byte, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, secret); err != nil {
		return nil, err
	}

	credentials, ok := secret.Data[key]
	if !ok {
		return nil, errors.New(fmt.Sprintf("key %s not found in secret %s/%s", key, namespace, name))
	}

	return credentials, nil
}

// writeCredentialsToFile writes the OCI credentials to a temporary file.
func writeCredentialsToFile(ctx context.Context, credentials []byte) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(2).Info("Writing credentials to file")
	credentialsFile, err := os.CreateTemp("", "oci-credentials-*.json")
	if err != nil {
		return "", err
	}

	if _, err := credentialsFile.Write(credentials); err != nil {
		return "", err
	}

	return credentialsFile.Name(), nil
}
