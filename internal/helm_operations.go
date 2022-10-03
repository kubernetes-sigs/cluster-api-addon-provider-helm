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
	"io/ioutil"
	"os"

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
	helmDriver "helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	addonsv1alpha1 "cluster-api-addon-provider-helm/api/v1alpha1"
)

func GetActionConfig(ctx context.Context, namespace string, config *rest.Config) (*helmAction.Configuration, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Getting action config")
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
	// Note: can change this to klog.V(4) or use a debug level
	if err := actionConfig.Init(cliConfig, namespace, "secret", klog.V(4).Infof); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

func HelmInit(ctx context.Context, namespace string, kubeconfig string) (*helmCli.EnvSettings, *helmAction.Configuration, error) {
	// log := ctrl.LoggerFrom(ctx)

	settings := helmCli.New()

	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, nil, err
	}

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

// Install Helm release if it doesn't exist. If it exists, check if it needs to be updated.
func InstallOrUpgradeHelmRelease(ctx context.Context, kubeconfig string, spec addonsv1alpha1.HelmReleaseProxySpec) (*release.Release, bool, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Installing or upgrading Helm release")

	// historyClient := helmAction.NewHistory(actionConfig)
	// historyClient.Max = 1
	// if _, err := historyClient.Run(spec.ReleaseName); err == helmDriver.ErrReleaseNotFound {
	existingRelease, err := GetHelmRelease(ctx, kubeconfig, spec)
	if err == helmDriver.ErrReleaseNotFound {
		release, err := InstallHelmRelease(ctx, kubeconfig, spec)
		if err != nil {
			return nil, false, err
		}
		return release, true, nil
	}

	return UpgradeHelmReleaseIfChanged(ctx, kubeconfig, spec, existingRelease)
}

func InstallHelmRelease(ctx context.Context, kubeconfig string, spec addonsv1alpha1.HelmReleaseProxySpec) (*release.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, kubeconfig)
	if err != nil {
		return nil, err
	}
	installClient := helmAction.NewInstall(actionConfig)
	installClient.RepoURL = spec.RepoURL
	installClient.Version = spec.Version
	installClient.Namespace = spec.ReleaseNamespace
	installClient.CreateNamespace = true

	if spec.ReleaseName == "" {
		installClient.GenerateName = true
		spec.ReleaseName, _, err = installClient.NameAndChart([]string{spec.ChartName})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate release name for chart %s on cluster %s", spec.ChartName, spec.ClusterRef.Name)
		}
	}
	installClient.ReleaseName = spec.ReleaseName

	log.V(2).Info("Locating chart...")
	cp, err := installClient.ChartPathOptions.LocateChart(spec.ChartName, settings)
	if err != nil {
		return nil, err
	}
	log.V(2).Info("Located chart at path", "path", cp)

	log.V(2).Info("Writing values to file")
	filename, err := writeValuesToFile(ctx, spec)
	if err != nil {
		return nil, err
	}
	defer os.Remove(filename)
	log.V(2).Info("Values written to file", "path", filename)
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	klog.V(2).Infof("Values written to file %s are:\n%s\n", filename, string(content))

	p := helmGetter.All(settings)
	valueOpts := &helmVals.Options{
		ValueFiles: []string{filename},
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
	release, err := installClient.RunWithContext(ctx, chartRequested, vals)
	if err != nil {
		return nil, err
	}

	log.V(2).Info("Released", "name", release.Name)

	return release, nil
}

// This function will be refactored to differentiate from installHelmRelease()
func UpgradeHelmReleaseIfChanged(ctx context.Context, kubeconfig string, spec addonsv1alpha1.HelmReleaseProxySpec, existing *release.Release) (*release.Release, bool, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, kubeconfig)
	if err != nil {
		return nil, false, err
	}
	upgradeClient := helmAction.NewUpgrade(actionConfig)
	upgradeClient.RepoURL = spec.RepoURL
	upgradeClient.Version = spec.Version
	upgradeClient.Namespace = spec.ReleaseNamespace
	log.V(2).Info("Locating chart...")
	cp, err := upgradeClient.ChartPathOptions.LocateChart(spec.ChartName, settings)
	if err != nil {
		return nil, false, err
	}
	log.V(2).Info("Located chart at path", "path", cp)

	log.V(2).Info("Writing values to file")
	filename, err := writeValuesToFile(ctx, spec)
	if err != nil {
		return nil, false, err
	}
	defer os.Remove(filename)
	log.V(2).Info("Values written to file", "path", filename)
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, false, err
	}

	klog.V(2).Infof("Values written to file %s are:\n%s\n", filename, string(content))

	p := helmGetter.All(settings)
	valueOpts := &helmVals.Options{
		ValueFiles: []string{filename},
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
	release, err := upgradeClient.RunWithContext(ctx, spec.ReleaseName, chartRequested, vals)
	if err != nil {
		return nil, false, err
	}

	return release, true, nil
}

func writeValuesToFile(ctx context.Context, spec addonsv1alpha1.HelmReleaseProxySpec) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(2).Info("Writing values to file")
	valuesFile, err := ioutil.TempFile("", spec.ChartName+"-"+spec.ReleaseName+"-*.yaml")
	if err != nil {
		return "", err
	}

	if _, err := valuesFile.Write([]byte(spec.Values)); err != nil {
		return "", err
	}

	return valuesFile.Name(), nil
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
	klog.V(2).Infof("Diff between values is:\n%s", cmp.Diff(existing.Config, values))

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

func GetHelmRelease(ctx context.Context, kubeconfig string, spec addonsv1alpha1.HelmReleaseProxySpec) (*release.Release, error) {
	if spec.ReleaseName == "" {
		return nil, helmDriver.ErrReleaseNotFound
	}

	_, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, kubeconfig)
	if err != nil {
		return nil, err
	}
	getClient := helmAction.NewGet(actionConfig)
	release, err := getClient.Run(spec.ReleaseName)
	if err != nil {
		return nil, err
	}

	return release, nil
}

func ListHelmReleases(ctx context.Context, kubeconfig string, spec addonsv1alpha1.HelmReleaseProxySpec) ([]*release.Release, error) {
	_, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, kubeconfig)
	if err != nil {
		return nil, err
	}
	listClient := helmAction.NewList(actionConfig)
	releases, err := listClient.Run()
	if err != nil {
		return nil, err
	}

	return releases, nil
}

func UninstallHelmRelease(ctx context.Context, kubeconfig string, spec addonsv1alpha1.HelmReleaseProxySpec) (*release.UninstallReleaseResponse, error) {
	_, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, kubeconfig)
	if err != nil {
		return nil, err
	}

	uninstallClient := helmAction.NewUninstall(actionConfig)
	response, err := uninstallClient.Run(spec.ReleaseName)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func RollbackHelmRelease(ctx context.Context, kubeconfig string, spec addonsv1alpha1.HelmReleaseProxySpec) error {
	_, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, kubeconfig)
	if err != nil {
		return err
	}

	rollbackClient := helmAction.NewRollback(actionConfig)
	return rollbackClient.Run(spec.ReleaseName)
}
