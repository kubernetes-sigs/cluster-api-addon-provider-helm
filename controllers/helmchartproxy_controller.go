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

	helmAction "helm.sh/helm/v3/pkg/action"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/cluster"
	// "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
)

// HelmChartProxyReconciler reconciles a HelmChartProxy object
type HelmChartProxyReconciler struct {
	client.Client
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

	log.Info("Getting list of clusters")
	clusterList, err := listClusters(ctx, r.Client)
	if err != nil {
		log.Error(err, "Failed to get list of clusters")
	}
	if clusterList == nil {
		log.Info("No clusters found")
	}
	for _, cluster := range clusterList.Items {
		log.Info("Found cluster", "name", cluster.Name)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if helmChartProxy.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("ENTERING DELETION TIMESTAMP")
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
			if err := r.reconcileDelete(ctx, helmChartProxy); err != nil {
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

	log.Info("Reconciling HelmChartProxy", "randomName", helmChartProxy.Name)
	err = r.reconcileNormal(ctx, helmChartProxy)
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

func listClusters(ctx context.Context, c client.Client) (*clusterv1.ClusterList, error) {
	clusterList := &clusterv1.ClusterList{}
	// labels := map[string]string{clusterv1.ClusterLabelName: name}

	if err := c.List(ctx, clusterList); err != nil {
		// if err := c.List(ctx, clusterList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return clusterList, nil
}

// reconcileNormal...
func (r *HelmChartProxyReconciler) reconcileNormal(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy) error {
	log := ctrl.LoggerFrom(ctx)

	releases, err := listHelmReleases(ctx)
	if err != nil {
		log.Info("Error listing releases", "err", err)
	}
	log.Info("Querying existing releases:")
	for _, release := range releases {
		log.Info("Release name", "name", release.Name)
	}

	existing, err := getHelmRelease(ctx, helmChartProxy.Spec)
	if err != nil {
		if err.Error() == "release: not found" {
			// Go ahead and create chart
			log.Info("Error getting chart:", err)
			release, err := installHelmRelease(ctx, helmChartProxy.Spec)
			if err != nil {
				log.Info("Error installing chart with Helm:", err)
				return err
			}
			if release != nil {
				log.Info(fmt.Sprintf("Release '%s' successfully installed\n", release.Name))
			}

			return nil
		}

		log.Info("Error getting chart:", err)
		return err
	}

	if existing != nil {
		// TODO: add logic for updating an existing release
		log.Info(fmt.Sprintf("Release '%s' already installed, running upgrade\n", existing.Name))
		release, err := upgradeHelmRelease(ctx, helmChartProxy.Spec)
		if err != nil {
			log.Info("Error upgrading chart with Helm:", err)
			return err
		}
		if release != nil {
			log.Info(fmt.Sprintf("Release '%s' successfully upgraded\n", release.Name))
		}
	}

	return nil
}

// func shouldUpdateHelmRelease(existing *release.Release, spec *addonsv1beta1.HelmChartProxySpec) bool {
// 	if existing == nil {
// 		return false
// 	}

// 	// if spec.RepoURL != existing.Chart.Repo {
// 	// 	return true
// 	// }
// 	return true
// }

// reconcileDelete...
func (r *HelmChartProxyReconciler) reconcileDelete(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy) error {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Prepared to uninstall chart spec", helmChartProxy.Spec)

	response, err := uninstallHelmRelease(ctx, helmChartProxy.Spec)
	if err != nil {
		log.Info("Error uninstalling chart with Helm:", err)
	}

	if response != nil && response.Info != "" {
		log.Info(fmt.Sprintf("Response is %s\n", response.Info))
	}

	return nil
}

func helmInit(ctx context.Context) (*helmCli.EnvSettings, *helmAction.Configuration, error) {
	log := ctrl.LoggerFrom(ctx)
	logf := func(format string, v ...interface{}) {
		log.Info(fmt.Sprintf(format, v...))
	}

	settings := helmCli.New()
	actionConfig := new(helmAction.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), "default", "secret", logf); err != nil {
		return nil, nil, err
	}

	return settings, actionConfig, nil
}

func installHelmRelease(ctx context.Context, spec addonsv1beta1.HelmChartProxySpec) (*release.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := helmInit(ctx)
	if err != nil {
		return nil, err
	}
	installer := helmAction.NewInstall(actionConfig)
	installer.RepoURL = spec.RepoURL
	installer.ReleaseName = spec.ReleaseName
	installer.Version = spec.Version
	installer.Namespace = "default"
	cp, err := installer.ChartPathOptions.LocateChart(spec.ChartName, settings)
	log.Info("Located chart at path", "path", cp)
	if err != nil {
		return nil, err
	}
	p := helmGetter.All(settings)
	specValues := make([]string, len(spec.Values))
	for k, v := range spec.Values {
		specValues = append(specValues, fmt.Sprintf("%s=%s", k, v))
	}
	valueOpts := &helmVals.Options{
		Values: specValues,
	}
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}
	chartRequested, err := helmLoader.Load(cp)

	if err != nil {
		return nil, err
	}
	log.Info("Installing with Helm...")
	release, err := installer.RunWithContext(ctx, chartRequested, vals)
	if err != nil {
		return nil, err
	}

	log.Info("Released", "name", release.Name)

	return release, nil
}

// This function will be refactored to differentiate from installHelmRelease()
func upgradeHelmRelease(ctx context.Context, spec addonsv1beta1.HelmChartProxySpec) (*release.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := helmInit(ctx)
	if err != nil {
		return nil, err
	}
	upgrader := helmAction.NewUpgrade(actionConfig)
	upgrader.RepoURL = spec.RepoURL
	upgrader.Version = spec.Version
	upgrader.Namespace = "default"
	cp, err := upgrader.ChartPathOptions.LocateChart(spec.ChartName, settings)
	log.Info("Located chart at path", "path", cp)
	if err != nil {
		return nil, err
	}
	p := helmGetter.All(settings)
	specValues := make([]string, len(spec.Values))
	for k, v := range spec.Values {
		specValues = append(specValues, fmt.Sprintf("%s=%s", k, v))
	}
	valueOpts := &helmVals.Options{
		Values: specValues,
	}
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}
	chartRequested, err := helmLoader.Load(cp)

	if err != nil {
		return nil, err
	}
	log.Info("Upgrading with Helm...")
	release, err := upgrader.RunWithContext(ctx, spec.ReleaseName, chartRequested, vals)
	if err != nil {
		return nil, err
	}

	return release, nil
}

func getHelmRelease(ctx context.Context, spec addonsv1beta1.HelmChartProxySpec) (*release.Release, error) {
	_, actionConfig, err := helmInit(ctx)
	if err != nil {
		return nil, err
	}
	getter := helmAction.NewGet(actionConfig)
	release, err := getter.Run(spec.ReleaseName)
	if err != nil {
		return nil, err
	}

	return release, nil
}

func listHelmReleases(ctx context.Context) ([]*release.Release, error) {
	_, actionConfig, err := helmInit(ctx)
	if err != nil {
		return nil, err
	}
	lister := helmAction.NewList(actionConfig)
	releases, err := lister.Run()
	if err != nil {
		return nil, err
	}

	return releases, nil
}

func uninstallHelmRelease(ctx context.Context, spec addonsv1beta1.HelmChartProxySpec) (*release.UninstallReleaseResponse, error) {
	_, actionConfig, err := helmInit(ctx)
	if err != nil {
		return nil, err
	}

	uninstaller := helmAction.NewUninstall(actionConfig)
	response, err := uninstaller.Run(spec.ReleaseName)
	if err != nil {
		return nil, err
	}

	return response, nil
}
