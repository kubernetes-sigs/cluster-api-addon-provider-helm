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

package internal

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	helmAction "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	ctrl "sigs.k8s.io/controller-runtime"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"
)

func HelmInit(ctx context.Context, kubeconfigPath string) (*helmCli.EnvSettings, *helmAction.Configuration, error) {
	log := ctrl.LoggerFrom(ctx)
	logf := func(format string, v ...interface{}) {
		log.V(4).Info(fmt.Sprintf(format, v...))
	}

	settings := helmCli.New()
	settings.KubeConfig = kubeconfigPath
	actionConfig := new(helmAction.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), "default", "secret", logf); err != nil {
		return nil, nil, err
	}

	return settings, actionConfig, nil
}

func InstallHelmRelease(ctx context.Context, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec, parsedValues []string) (*release.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := HelmInit(ctx, kubeconfigPath)
	if err != nil {
		return nil, err
	}
	installer := helmAction.NewInstall(actionConfig)
	installer.RepoURL = spec.RepoURL
	installer.ReleaseName = spec.ReleaseName
	installer.Version = spec.Version
	installer.Namespace = "default"
	cp, err := installer.ChartPathOptions.LocateChart(spec.ChartName, settings)
	log.V(2).Info("Located chart at path", "path", cp)
	if err != nil {
		return nil, err
	}
	p := helmGetter.All(settings)
	valueOpts := &helmVals.Options{
		Values: parsedValues,
	}
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}
	chartRequested, err := helmLoader.Load(cp)

	if err != nil {
		return nil, err
	}
	log.V(2).Info("Installing with Helm...")
	release, err := installer.RunWithContext(ctx, chartRequested, vals)
	if err != nil {
		return nil, err
	}

	log.V(2).Info("Released", "name", release.Name)

	return release, nil
}

// This function will be refactored to differentiate from installHelmRelease()
func UpgradeHelmRelease(ctx context.Context, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec, parsedValues []string) (*release.Release, bool, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := HelmInit(ctx, kubeconfigPath)
	if err != nil {
		return nil, false, err
	}
	upgrader := helmAction.NewUpgrade(actionConfig)
	upgrader.RepoURL = spec.RepoURL
	upgrader.Version = spec.Version
	upgrader.Namespace = "default"
	cp, err := upgrader.ChartPathOptions.LocateChart(spec.ChartName, settings)
	log.V(2).Info("Located chart at path", "path", cp)
	if err != nil {
		return nil, false, err
	}
	p := helmGetter.All(settings)
	valueOpts := &helmVals.Options{
		Values: parsedValues,
	}
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, false, err
	}
	chartRequested, err := helmLoader.Load(cp)
	if err != nil {
		return nil, false, err
	}
	if chartRequested == nil {
		return nil, false, errors.Errorf("failed to load request chart %s", spec.ChartName)
	}

	existing, err := GetHelmRelease(ctx, kubeconfigPath, spec)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get existing release")
	}

	shouldUpgrade, err := shouldUpgradeHelmRelease(ctx, *existing, *chartRequested)
	if err != nil {
		return nil, false, err
	}
	if !shouldUpgrade {
		log.V(2).Info(fmt.Sprintf("Release `%s` is up to date, no upgrade requried", spec.ReleaseName))
		return existing, false, nil
	}

	log.V(2).Info(fmt.Sprintf("Upgrading release `%s` with Helm", spec.ReleaseName))
	release, err := upgrader.RunWithContext(ctx, spec.ReleaseName, chartRequested, vals)
	if err != nil {
		return nil, false, err
	}

	return release, true, nil
}

func shouldUpgradeHelmRelease(ctx context.Context, existing release.Release, chartRequested chart.Chart) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	if existing.Chart == nil || existing.Chart.Metadata == nil {
		return false, errors.New("Failed to resolve chart version of existing release")
	}
	if chartRequested.Metadata == nil {
		return false, errors.New("Failed to resolve chart version of from helm repo")
	}

	log.V(3).Info("[TESTING] Comparing releases", "releasesAreEqual", reflect.DeepEqual(existing.Chart, chartRequested))

	if existing.Chart.Metadata.Version != chartRequested.Metadata.Version {
		log.V(3).Info("Values are different, upgrading")
		return true, nil
	}

	log.V(3).Info("Existing chart values are", "values", existing.Chart.Values)
	log.V(3).Info("Requested chart values are", "values", chartRequested.Values)

	return !reflect.DeepEqual(existing.Chart.Values, chartRequested.Values), nil
}

func GetHelmRelease(ctx context.Context, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec) (*release.Release, error) {
	_, actionConfig, err := HelmInit(ctx, kubeconfigPath)
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

func ListHelmReleases(ctx context.Context, kubeconfigPath string) ([]*release.Release, error) {
	_, actionConfig, err := HelmInit(ctx, kubeconfigPath)
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

func UninstallHelmRelease(ctx context.Context, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec) (*release.UninstallReleaseResponse, error) {
	_, actionConfig, err := HelmInit(ctx, kubeconfigPath)
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
