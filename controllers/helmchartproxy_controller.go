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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"
	"cluster-api-addon-helm/internal"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/cluster"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
)

// HelmChartProxyReconciler reconciles a HelmChartProxy object
type HelmChartProxyReconciler struct {
	ctrlClient.Client
	Scheme *runtime.Scheme
}

const finalizer = "addons.cluster.x-k8s.io"

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
func (r *HelmChartProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the HelmChartProxy instance.
	helmChartProxy := &addonsv1beta1.HelmChartProxy{}
	if err := r.Client.Get(ctx, req.NamespacedName, helmChartProxy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	defer func() {
		reterr = r.Status().Update(context.TODO(), helmChartProxy)
	}()

	labelSelector := helmChartProxy.Spec.Selector
	log.V(2).Info("HelmChartProxy labels are", "labels", labelSelector)

	log.V(2).Info("Getting list of clusters with labels")
	clusterList, err := r.listClustersWithLabels(ctx, labelSelector)
	if err != nil {
		// helmChartProxy.Status.FailureReason = errors.Wrapf(err, "failed to get list of clusters")
		return ctrl.Result{}, err
	}
	if clusterList == nil {
		log.V(2).Info("No clusters found")
	}
	for _, cluster := range clusterList.Items {
		log.V(2).Info("Found cluster", "name", cluster.Name)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if helmChartProxy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(helmChartProxy, finalizer) {
			controllerutil.AddFinalizer(helmChartProxy, finalizer)
			if err := r.Update(ctx, helmChartProxy); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(helmChartProxy, finalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.reconcileDelete(ctx, helmChartProxy, clusterList.Items); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				helmChartProxy.Status.Ready = false
				return ctrl.Result{}, err
			}

			helmChartProxy.Status.Ready = true
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(helmChartProxy, finalizer)
			if err := r.Update(ctx, helmChartProxy); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	log.V(2).Info("Reconciling HelmChartProxy", "randomName", helmChartProxy.Name)
	err = r.reconcileNormal(ctx, helmChartProxy, clusterList.Items)
	if err != nil {
		helmChartProxy.Status.Ready = false
		// helmChartProxy.Status.FailureReason = err.Error()
		return ctrl.Result{}, err
	}

	helmChartProxy.Status.Ready = true
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmChartProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1beta1.HelmChartProxy{}).
		Complete(r)
}

// reconcileNormal...
func (r *HelmChartProxyReconciler) reconcileNormal(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, clusters []clusterv1.Cluster) error {
	log := ctrl.LoggerFrom(ctx)

	for _, cluster := range clusters {
		kubeconfigPath, err := internal.WriteClusterKubeconfigToFile(ctx, &cluster)
		if err != nil {
			log.Error(err, "failed to get kubeconfig for cluster", "cluster", cluster.Name)
			return err
		}

		err = r.reconcileCluster(ctx, helmChartProxy, &cluster, kubeconfigPath)
		if err != nil {
			log.Error(err, "failed to reconcile chart on cluster", "cluster", cluster.Name)
			return err
		}
	}

	return nil
}

func (r *HelmChartProxyReconciler) reconcileCluster(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, cluster *clusterv1.Cluster, kubeconfigPath string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Reconciling HelmChartProxy", "proxy", helmChartProxy.Name, "on cluster", "name", cluster.Name)

	releases, err := internal.ListHelmReleases(ctx, kubeconfigPath)
	if err != nil {
		log.V(2).Info("Error listing releases", "err", err)
	}
	log.V(2).Info("Querying existing releases:")
	for _, release := range releases {
		log.V(2).Info("Release name", "name", release.Name, "installed on", "cluster", cluster.Name)
	}

	values, err := internal.ParseValues(ctx, r.Client, kubeconfigPath, helmChartProxy.Spec, cluster)
	if err != nil {
		log.Error(err, "failed to parse values", "cluster", cluster.Name)
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
				log.V(2).Info((fmt.Sprintf("Release '%s' successfully installed on cluster %s\n", release.Name, cluster.Name)))
				// addClusterRefToStatusList(ctx, helmChartProxy, cluster)
			}

			return nil
		}

		return err
	}

	if existing != nil {
		// TODO: add logic for updating an existing release
		log.V(2).Info(fmt.Sprintf("Release '%s' already installed on cluster %s, running upgrade\n", existing.Name, cluster.Name))
		release, upgraded, err := internal.UpgradeHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec, values)
		if err != nil {
			log.V(2).Error(err, "error upgrading chart with Helm on cluster", "cluster", cluster.Name)
			return errors.Wrapf(err, "error upgrading chart with Helm on cluster %s", cluster.Name)
		}
		if release != nil && upgraded {
			log.V(2).Info((fmt.Sprintf("Release '%s' successfully upgraded on cluster %s\n", release.Name, cluster.Name)))
			// addClusterRefToStatusList(ctx, helmChartProxy, cluster)
		}
	}

	return nil
}

// reconcileDelete...
func (r *HelmChartProxyReconciler) reconcileDelete(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, clusters []clusterv1.Cluster) error {
	log := ctrl.LoggerFrom(ctx)

	for _, cluster := range clusters {
		kubeconfigPath, err := internal.WriteClusterKubeconfigToFile(ctx, &cluster)
		if err != nil {
			log.Error(err, "failed to get kubeconfig for cluster", "cluster", cluster.Name)
			return errors.Wrapf(err, "failed to get kubeconfig for cluster %s", cluster.Name)
		}
		err = r.reconcileDeleteCluster(ctx, helmChartProxy, &cluster, kubeconfigPath)
		if err != nil {
			log.Error(err, "failed to delete chart on cluster", "cluster", cluster.Name)
			return err
		}
	}

	return nil
}

func (r *HelmChartProxyReconciler) reconcileDeleteCluster(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, clusters *clusterv1.Cluster, kubeconfigPath string) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Prepared to uninstall chart spec", "spec", helmChartProxy.Name, "on cluster", "name", clusters.Name)

	response, err := internal.UninstallHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec)
	if err != nil {
		log.V(2).Info("Error uninstalling chart with Helm:", err)
		return errors.Wrapf(err, "error uninstalling chart with Helm on cluster %s", clusters.Name)
	}

	log.V(2).Info("Successfully uninstalled chart spec", "spec", helmChartProxy.Name, "on cluster", "name", clusters.Name)
	if response != nil && response.Info != "" {
		log.V(2).Info(fmt.Sprintf("Response is %s\n", response.Info))
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
