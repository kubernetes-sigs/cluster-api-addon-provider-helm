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
	"os"
	"time"

	helmAction "helm.sh/helm/v3/pkg/action"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"
)

// HelmChartProxyReconciler reconciles a HelmChartProxy object
type HelmChartProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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
	log := ctrlLog.FromContext(ctx)
	// log := ctrl.LoggerFrom(ctx)

	// Fetch the HelmChartProxy instance.
	helmChartProxy := &addonsv1beta1.HelmChartProxy{}
	if err := r.Client.Get(ctx, req.NamespacedName, helmChartProxy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info(fmt.Sprintf("Got a new HelmChartProxy: %s", helmChartProxy.Name))
	log.Info(fmt.Sprintf("HelmChartProxy spec is %+v", helmChartProxy.Spec))

	settings := helmCli.New()
	actionConfig := new(helmAction.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), "default", os.Getenv("HELM_DRIVER"), Logf); err != nil {
		return ctrl.Result{}, err
	}
	i := helmAction.NewInstall(actionConfig)
	i.RepoURL = helmChartProxy.Spec.RepoURL
	i.ReleaseName = helmChartProxy.Spec.ChartName
	cp, err := i.ChartPathOptions.LocateChart(helmChartProxy.Spec.ChartReference, settings)
	log.Info(fmt.Sprintf("Chart path is %s", cp))
	if err != nil {
		return ctrl.Result{}, err
	}
	p := helmGetter.All(settings)
	valueOpts := &helmVals.Options{}
	// valueOpts.Values = []string{fmt.Sprintf("infra.clusterName=%s", input.ClusterProxy.GetName())}
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return ctrl.Result{}, err
	}
	chartRequested, err := helmLoader.Load(cp)
	chartRequested.Metadata.Name = helmChartProxy.Spec.ChartName
	log.Info(fmt.Sprintf("Requested chart is %+v", *chartRequested))
	log.Info(fmt.Sprintf("Chart name is %s", chartRequested.Name()))
	log.Info(fmt.Sprintf("Values are is %+v", vals))

	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Running with context")
	release, err := i.RunWithContext(ctx, chartRequested, vals)
	if err != nil {
		log.Info("Error running with context")
		return ctrl.Result{}, err
	}

	log.Info(fmt.Sprintf("Release %s is %+v", release.Name, release))
	log.Info(fmt.Sprintf("Release chart is %+v", *release.Chart))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmChartProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1beta1.HelmChartProxy{}).
		Complete(r)
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func logf_helper(level string, format string, args ...interface{}) {
	fmt.Printf(nowStamp()+": "+level+": "+format+"\n", args...)
}

// Logf prints info logs with a timestamp and formatting.
func Logf(format string, args ...interface{}) {
	logf_helper("INFO", format, args...)
}
