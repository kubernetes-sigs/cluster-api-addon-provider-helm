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
	"log"
	"os"

	helmAction "helm.sh/helm/v3/pkg/action"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"
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
	_ = ctrlLog.FromContext(ctx)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Fetch the HelmChartProxy instance.
	helmChartProxy := &addonsv1beta1.HelmChartProxy{}
	if err := r.Client.Get(ctx, req.NamespacedName, helmChartProxy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
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

	err := r.reconcileNormal(ctx, helmChartProxy)
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

// reconcileNormal...
func (r *HelmChartProxyReconciler) reconcileNormal(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy) error {
	if _, err := getHelmRelease(ctx, helmChartProxy.Spec); err != nil {
		log.Println("Error getting chart:", err)
	}

	// TODO: Handle error when it's run on an existing release.
	if _, err := installHelmRelease(ctx, helmChartProxy.Spec); err != nil {
		log.Println("Error installing chart with Helm:", err)
		return err
	}

	releases, err := listHelmReleases(ctx)
	if err != nil {
		log.Println("Error listing charts with Helm:", err)
		return err
	}

	log.Println("Helm releases:")
	limit := 200
	for i, release := range releases {
		log.Printf("%d. %s:\n", i, release.Name)
		if len(release.Manifest) > limit {
			log.Printf("%s\n...\n", release.Manifest[0:limit])
		} else {
			log.Println(release.Manifest)
		}

	}

	return nil
}

// reconcileDelete...
func (r *HelmChartProxyReconciler) reconcileDelete(ctx context.Context, helmChartProxy *addonsv1beta1.HelmChartProxy) error {
	log.Println("Prepared to uninstall chart spec", helmChartProxy.Spec)

	response, err := uninstallHelmRelease(ctx, helmChartProxy.Spec)
	if err != nil {
		log.Println("Error uninstalling chart with Helm:", err)
	}

	if response != nil && response.Info != "" {
		log.Printf("Response is %s\n", response.Info)
	}

	return nil
}

func helmInit() (*helmCli.EnvSettings, *helmAction.Configuration, error) {
	settings := helmCli.New()
	actionConfig := new(helmAction.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), "default", os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, nil, err
	}

	return settings, actionConfig, nil
}

func installHelmRelease(ctx context.Context, spec addonsv1beta1.HelmChartProxySpec) (*release.Release, error) {
	settings, actionConfig, err := helmInit()
	if err != nil {
		return nil, err
	}
	installer := helmAction.NewInstall(actionConfig)
	installer.RepoURL = spec.RepoURL
	installer.ReleaseName = spec.ReleaseName
	cp, err := installer.ChartPathOptions.LocateChart(spec.ChartReference, settings)
	log.Println("Located chart at path", cp)
	if err != nil {
		return nil, err
	}
	p := helmGetter.All(settings)
	valueOpts := &helmVals.Options{}
	// valueOpts.Values = []string{fmt.Printf("infra.clusterName=%s", input.ClusterProxy.GetName())
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}
	chartRequested, err := helmLoader.Load(cp)
	// chartRequested.Metadata.Name = spec.ReleaseName // TODO: remove this if not needed

	if err != nil {
		return nil, err
	}
	log.Println("Installing with Helm...")
	release, err := installer.RunWithContext(ctx, chartRequested, vals)
	if err != nil {
		return nil, err
	}

	log.Println("Released", release.Name)

	return release, nil
}

func getHelmRelease(ctx context.Context, spec addonsv1beta1.HelmChartProxySpec) (*release.Release, error) {
	_, actionConfig, err := helmInit()
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
	_, actionConfig, err := helmInit()
	if err != nil {
		return nil, err
	}
	lister := helmAction.NewList(actionConfig)
	releases, err := lister.Run()
	if err != nil {
		log.Println("Error listing releases with Helm:", err)
		return nil, err
	}

	return releases, nil
}

func uninstallHelmRelease(ctx context.Context, spec addonsv1beta1.HelmChartProxySpec) (*release.UninstallReleaseResponse, error) {
	_, actionConfig, err := helmInit()
	if err != nil {
		return nil, err
	}

	uninstaller := helmAction.NewUninstall(actionConfig)
	response, err := uninstaller.Run(spec.ReleaseName)
	if err != nil {
		log.Println("Failed to uninstall chart", spec.ReleaseName)
		log.Println(err.Error())
		return nil, err
	}

	return response, nil
}
