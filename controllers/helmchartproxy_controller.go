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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"

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
func (r *HelmChartProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the HelmChartProxy instance.
	helmChartProxy := &addonsv1beta1.HelmChartProxy{}
	if err := r.Client.Get(ctx, req.NamespacedName, helmChartProxy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	labels := helmChartProxy.GetLabels()
	log.V(2).Info("HelmChartProxy labels are", "labels", labels)

	log.V(2).Info("Getting list of clusters with labels")
	clusterList, err := listClustersWithLabels(ctx, r.Client, labels)
	if err != nil {
		log.Error(err, "Failed to get list of clusters")
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
				return ctrl.Result{}, err
			}

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
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmChartProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1beta1.HelmChartProxy{}).
		Complete(r)
}

func listClustersWithLabels(ctx context.Context, c ctrlClient.Client, labels map[string]string) (*clusterv1.ClusterList, error) {
	clusterList := &clusterv1.ClusterList{}
	// labels := map[string]string{clusterv1.ClusterLabelName: name}

	if err := c.List(ctx, clusterList, ctrlClient.MatchingLabels(labels)); err != nil {
		// if err := c.List(ctx, clusterList, ctrlClient.InNamespace(namespace), ctrlClient.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return clusterList, nil
}

// reconcileNormal...
func (r *HelmChartProxyReconciler) reconcileNormal(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, clusters []clusterv1.Cluster) error {
	log := ctrl.LoggerFrom(ctx)

	for _, cluster := range clusters {
		kubeconfigPath, err := writeClusterKubeconfigToFile(ctx, &cluster)
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

	releases, err := listHelmReleases(ctx, kubeconfigPath)
	if err != nil {
		log.V(2).Info("Error listing releases", "err", err)
	}
	log.V(2).Info("Querying existing releases:")
	for _, release := range releases {
		log.V(2).Info("Release name", "name", release.Name, "installed on", "cluster", cluster.Name)
	}

	existing, err := getHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec)
	if err != nil {
		log.V(2).Error(err, "error getting release from cluster", "cluster", cluster.Name)

		if err.Error() == "release: not found" {
			// Go ahead and create chart
			release, err := installHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec)
			if err != nil {
				log.V(2).Error(err, "error installing chart with Helm on cluster", "cluster", cluster.Name)
				return err
			}
			if release != nil {
				log.V(2).Info((fmt.Sprintf("Release '%s' successfully installed on cluster %s\n", release.Name, cluster.Name)))
			}

			return nil
		}

		return err
	}

	if existing != nil {
		// TODO: add logic for updating an existing release
		log.V(2).Info(fmt.Sprintf("Release '%s' already installed on cluster %s, running upgrade\n", existing.Name, cluster.Name))
		release, err := upgradeHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec)
		if err != nil {
			log.V(2).Error(err, "error upgrading chart with Helm on cluster", "cluster", cluster.Name)
			return err
		}
		if release != nil {
			log.V(2).Info((fmt.Sprintf("Release '%s' successfully upgraded on cluster %s\n", release.Name, cluster.Name)))
		}
	}

	return nil
}

// reconcileDelete...
func (r *HelmChartProxyReconciler) reconcileDelete(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy, clusters []clusterv1.Cluster) error {
	log := ctrl.LoggerFrom(ctx)

	for _, cluster := range clusters {
		kubeconfigPath, err := writeClusterKubeconfigToFile(ctx, &cluster)
		if err != nil {
			log.Error(err, "failed to get kubeconfig for cluster", "cluster", cluster.Name)
			return err
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

	response, err := uninstallHelmRelease(ctx, kubeconfigPath, helmChartProxy.Spec)
	if err != nil {
		log.V(2).Info("Error uninstalling chart with Helm:", err)
	}

	log.V(2).Info("Successfully uninstalled chart spec", "spec", helmChartProxy.Name, "on cluster", "name", clusters.Name)
	if response != nil && response.Info != "" {
		log.V(2).Info(fmt.Sprintf("Response is %s\n", response.Info))
	}

	return nil
}
