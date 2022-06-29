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

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	helmAction "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"
)

func GetActionConfig(ctx context.Context, namespace string, config *rest.Config) (*helmAction.Configuration, error) {
	log := ctrl.LoggerFrom(ctx)
	logf := func(format string, v ...interface{}) {
		log.V(4).Info(fmt.Sprintf(format, v...))
	}

	actionConfig := new(helmAction.Configuration)
	// var cliConfig *genericclioptions.ConfigFlags
	// cliConfig := &genericclioptions.ConfigFlags{
	// 	Namespace:        &env.namespace,
	// 	Context:          &env.KubeContext,
	// 	BearerToken:      &env.KubeToken,
	// 	APIServer:        &env.KubeAPIServer,
	// 	CAFile:           &env.KubeCaFile,
	// 	KubeConfig:       &env.KubeConfig,
	// 	Impersonate:      &env.KubeAsUser,
	// 	ImpersonateGroup: &env.KubeAsGroups,
	// }
	insecure := true
	cliConfig := genericclioptions.NewConfigFlags(false)
	cliConfig.APIServer = &config.Host
	cliConfig.BearerToken = &config.BearerToken
	cliConfig.Namespace = &namespace
	cliConfig.Insecure = &insecure
	// Drop their rest.Config and just return inject own
	wrapper := func(*rest.Config) *rest.Config {
		return config
	}
	cliConfig.WithWrapConfigFn(wrapper)
	// cliConfig.Insecure = &insecure
	if err := actionConfig.Init(cliConfig, namespace, "secret", logf); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

func HelmInit(ctx context.Context, kubeconfig string) (*helmCli.EnvSettings, *helmAction.Configuration, error) {
	// log := ctrl.LoggerFrom(ctx)

	settings := helmCli.New()

	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, nil, err
	}

	namespace := "default"
	actionConfig, err := GetActionConfig(ctx, namespace, restConfig)
	if err != nil {
		return nil, nil, err
	}

	// settings.KubeConfig = kubeconfig
	// actionConfig := new(helmAction.Configuration)
	// if err := actionConfig.Init(settings.RESTClientGetter(), "default", "secret", logf); err != nil {
	// 	return nil, nil, err
	// }

	return settings, actionConfig, nil
}

func InstallHelmRelease(ctx context.Context, kubeconfig string, spec addonsv1beta1.HelmReleaseProxySpec, parsedValues []string) (*release.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := HelmInit(ctx, kubeconfig)
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
func UpgradeHelmRelease(ctx context.Context, kubeconfig string, spec addonsv1beta1.HelmReleaseProxySpec, parsedValues []string) (*release.Release, bool, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := HelmInit(ctx, kubeconfig)
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

	existing, err := GetHelmRelease(ctx, kubeconfig, spec)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get existing release")
	}

	shouldUpgrade, err := shouldUpgradeHelmRelease(ctx, *existing, chartRequested, vals)
	if err != nil {
		return nil, false, err
	}
	if !shouldUpgrade {
		log.V(2).Info(fmt.Sprintf("Release `%s` is up to date, no upgrade requried, revision = %d", existing.Name, existing.Version))
		return existing, false, nil
	}

	log.V(2).Info(fmt.Sprintf("Upgrading release `%s` with Helm", spec.ReleaseName))
	// upgrader.DryRun = true
	release, err := upgrader.RunWithContext(ctx, spec.ReleaseName, chartRequested, vals)
	if err != nil {
		return nil, false, err
	}
	fmt.Printf("DryRun config diff: %s", cmp.Diff(release.Config, existing.Config))

	return release, true, nil
}

func shouldUpgradeHelmRelease(ctx context.Context, existing release.Release, chartRequested *chart.Chart, values map[string]interface{}) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	if existing.Chart == nil || existing.Chart.Metadata == nil {
		return false, errors.New("Failed to resolve chart version of existing release")
	}
	if existing.Chart.Metadata.Version != chartRequested.Metadata.Version {
		log.V(3).Info("Versions are different, upgrading")
		return true, nil
	}
	fmt.Printf("Got diff: %s", cmp.Diff(existing.Config, values))

	// TODO: Comparing yaml is not ideal, but it's the best we can do since DeepEquals fails. This is because int64 types
	// are converted to float64 when returned from the helm API.
	oldValues, err := yaml.Marshal(existing.Config)
	if err != nil {
		return false, errors.Wrapf(err, "failed to marshal existing release values")
	}
	newValues, err := yaml.Marshal(values)
	if err != nil {
		return false, errors.Wrapf(err, "failed to new release values")
	}

	return !cmp.Equal(oldValues, newValues), nil
}

func GetHelmRelease(ctx context.Context, kubeconfig string, spec addonsv1beta1.HelmReleaseProxySpec) (*release.Release, error) {
	_, actionConfig, err := HelmInit(ctx, kubeconfig)
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

func ListHelmReleases(ctx context.Context, kubeconfig string) ([]*release.Release, error) {
	_, actionConfig, err := HelmInit(ctx, kubeconfig)
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

func UninstallHelmRelease(ctx context.Context, kubeconfig string, spec addonsv1beta1.HelmReleaseProxySpec) (*release.UninstallReleaseResponse, error) {
	_, actionConfig, err := HelmInit(ctx, kubeconfig)
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
