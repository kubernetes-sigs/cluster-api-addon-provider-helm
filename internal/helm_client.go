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

package internal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	helmAction "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	helmRelease "helm.sh/helm/v3/pkg/release"
	helmDriver "helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Client interface {
	InstallOrUpgradeHelmRelease(ctx context.Context, restConfig *rest.Config, credentialsPath, caFilePath string, spec addonsv1alpha1.HelmReleaseProxySpec) (*helmRelease.Release, error)
	GetHelmRelease(ctx context.Context, restConfig *rest.Config, spec addonsv1alpha1.HelmReleaseProxySpec) (*helmRelease.Release, error)
	UninstallHelmRelease(ctx context.Context, restConfig *rest.Config, spec addonsv1alpha1.HelmReleaseProxySpec) (*helmRelease.UninstallReleaseResponse, error)
}

var _ Client = (*HelmClient)(nil)

type HelmClient struct{}

// GetActionConfig returns a new Helm action configuration.
func GetActionConfig(ctx context.Context, namespace string, config *rest.Config) (*helmAction.Configuration, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Getting action config")
	actionConfig := new(helmAction.Configuration)
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

// HelmInit initializes Helm.
func HelmInit(ctx context.Context, namespace string, restConfig *rest.Config) (*helmCli.EnvSettings, *helmAction.Configuration, error) {
	// log := ctrl.LoggerFrom(ctx)

	settings := helmCli.New()

	actionConfig, err := GetActionConfig(ctx, namespace, restConfig)
	if err != nil {
		return nil, nil, err
	}

	return settings, actionConfig, nil
}

// InstallOrUpgradeHelmRelease installs a Helm release if it does not exist, or upgrades it if it does and differs from the spec.
// It returns a boolean indicating whether an install or upgrade was performed.
func (c *HelmClient) InstallOrUpgradeHelmRelease(ctx context.Context, restConfig *rest.Config, credentialsPath, caFilePath string, spec addonsv1alpha1.HelmReleaseProxySpec) (*helmRelease.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Installing or upgrading Helm release")

	// historyClient := helmAction.NewHistory(actionConfig)
	// historyClient.Max = 1
	// if _, err := historyClient.Run(spec.ReleaseName); err == helmDriver.ErrReleaseNotFound {
	existingRelease, err := c.GetHelmRelease(ctx, restConfig, spec)
	if err != nil {
		if errors.Is(err, helmDriver.ErrReleaseNotFound) {
			return c.InstallHelmRelease(ctx, restConfig, credentialsPath, caFilePath, spec)
		}

		return nil, err
	}

	return c.UpgradeHelmReleaseIfChanged(ctx, restConfig, credentialsPath, caFilePath, spec, existingRelease)
}

// generateHelmInstallConfig generates default helm install config using helmOptions specified in HCP CR spec.
func generateHelmInstallConfig(actionConfig *helmAction.Configuration, helmOptions *addonsv1alpha1.HelmOptions) *helmAction.Install {
	installClient := helmAction.NewInstall(actionConfig)
	installClient.CreateNamespace = true
	if actionConfig.RegistryClient != nil {
		installClient.SetRegistryClient(actionConfig.RegistryClient)
	}
	if helmOptions == nil {
		return installClient
	}

	installClient.DisableHooks = helmOptions.DisableHooks
	installClient.Wait = helmOptions.Wait
	installClient.WaitForJobs = helmOptions.WaitForJobs
	if helmOptions.Timeout != nil {
		installClient.Timeout = helmOptions.Timeout.Duration
	}
	installClient.SkipCRDs = helmOptions.SkipCRDs
	installClient.SubNotes = helmOptions.SubNotes
	installClient.DisableOpenAPIValidation = helmOptions.DisableOpenAPIValidation
	installClient.Atomic = helmOptions.Atomic
	installClient.IncludeCRDs = helmOptions.Install.IncludeCRDs
	installClient.CreateNamespace = helmOptions.Install.CreateNamespace

	return installClient
}

// generateHelmUpgradeConfig generates default helm upgrade config using helmOptions specified in HCP CR spec.
func generateHelmUpgradeConfig(actionConfig *helmAction.Configuration, helmOptions *addonsv1alpha1.HelmOptions) *helmAction.Upgrade {
	upgradeClient := helmAction.NewUpgrade(actionConfig)
	if actionConfig.RegistryClient != nil {
		upgradeClient.SetRegistryClient(actionConfig.RegistryClient)
	}
	if helmOptions == nil {
		return upgradeClient
	}

	upgradeClient.DisableHooks = helmOptions.DisableHooks
	upgradeClient.Wait = helmOptions.Wait
	upgradeClient.WaitForJobs = helmOptions.WaitForJobs
	if helmOptions.Timeout != nil {
		upgradeClient.Timeout = helmOptions.Timeout.Duration
	}
	upgradeClient.SkipCRDs = helmOptions.SkipCRDs
	upgradeClient.SubNotes = helmOptions.SubNotes
	upgradeClient.DisableOpenAPIValidation = helmOptions.DisableOpenAPIValidation
	upgradeClient.Atomic = helmOptions.Atomic
	upgradeClient.Force = helmOptions.Upgrade.Force
	upgradeClient.ResetValues = helmOptions.Upgrade.ResetValues
	upgradeClient.ReuseValues = helmOptions.Upgrade.ReuseValues
	upgradeClient.ResetThenReuseValues = helmOptions.Upgrade.ResetThenReuseValues
	upgradeClient.MaxHistory = helmOptions.Upgrade.MaxHistory
	upgradeClient.CleanupOnFail = helmOptions.Upgrade.CleanupOnFail

	return upgradeClient
}

// InstallHelmRelease installs a Helm release.
func (c *HelmClient) InstallHelmRelease(ctx context.Context, restConfig *rest.Config, credentialsPath, caFilePath string, spec addonsv1alpha1.HelmReleaseProxySpec) (*helmRelease.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, restConfig)
	if err != nil {
		return nil, err
	}
	settings.RegistryConfig = credentialsPath

	enableClientCache := spec.Options.EnableClientCache
	log.V(2).Info("Creating install registry client", "enableClientCache", enableClientCache, "credentialsPath", credentialsPath, "cFilePath", caFilePath)

	registryClient, err := newDefaultRegistryClient(credentialsPath, enableClientCache, caFilePath, ptr.Deref(spec.TLSConfig, addonsv1alpha1.TLSConfig{}).InsecureSkipTLSVerify)
	if err != nil {
		return nil, err
	}

	actionConfig.RegistryClient = registryClient

	chartName, repoURL, err := getHelmChartAndRepoName(spec.ChartName, spec.RepoURL)
	if err != nil {
		return nil, err
	}

	installClient := generateHelmInstallConfig(actionConfig, &spec.Options)
	installClient.RepoURL = repoURL
	installClient.Version = spec.Version
	installClient.Namespace = spec.ReleaseNamespace

	if spec.ReleaseName == "" {
		installClient.GenerateName = true
		spec.ReleaseName, _, err = installClient.NameAndChart([]string{chartName})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate release name for chart %s on cluster %s", spec.ChartName, spec.ClusterRef.Name)
		}
	}
	installClient.ReleaseName = spec.ReleaseName

	log.V(2).Info("Locating chart...")
	cp, err := installClient.ChartPathOptions.LocateChart(chartName, settings)
	if err != nil {
		return nil, err
	}
	log.V(2).Info("Located chart at path", "path", cp)

	log.V(2).Info("Writing values to file")
	filename, err := writeValuesToFile(ctx, spec)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := os.Remove(filename)
		if err != nil {
			log.Error(err, "Cannot remove file", "path", filename)
		}
	}()

	log.V(2).Info("Values written to file", "path", filename)
	content, err := os.ReadFile(filename)
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
	log.V(1).Info("Installing with Helm", "chart", spec.ChartName, "repo", spec.RepoURL)

	return installClient.RunWithContext(ctx, chartRequested, vals) // Can return error and a release
}

// newDefaultRegistryClient creates registry client object with default config which can be used to install/upgrade helm charts.
func newDefaultRegistryClient(credentialsPath string, enableCache bool, caFilePath string, insecureSkipTLSVerify bool) (*registry.Client, error) {
	opts := []registry.ClientOption{
		registry.ClientOptDebug(true),
		registry.ClientOptEnableCache(enableCache),
		registry.ClientOptWriter(os.Stderr),
	}
	if credentialsPath != "" {
		// Create a new registry client with credentials
		opts = append(opts, registry.ClientOptCredentialsFile(credentialsPath))
	}

	if caFilePath != "" || insecureSkipTLSVerify {
		tlsConf, err := newClientTLS(caFilePath, insecureSkipTLSVerify)
		if err != nil {
			return nil, fmt.Errorf("can't create TLS config for client: %w", err)
		}
		opts = append(opts, registry.ClientOptHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConf,
				Proxy:           http.ProxyFromEnvironment,
				// This registry client is not reused and is discarded after a single reconciliation
				// loop. Limit how long can be the idle connection open. Otherwise its possible that
				// a registry server that keeps the connection open for a long time could result in
				// connections being openend in the controller for a long time.
				IdleConnTimeout: 1 * time.Second,
			},
		}),
		)
	}

	return registry.NewClient(opts...)
}

// newClientTLS creates a new TLS config for the registry client based on the provided
// CA file and insecureSkipTLSverify flag.
func newClientTLS(caFile string, insecureSkipTLSverify bool) (*tls.Config, error) {
	config := tls.Config{
		InsecureSkipVerify: insecureSkipTLSverify,
	}

	if caFile != "" {
		cp, err := certPoolFromFile(caFile)
		if err != nil {
			return nil, err
		}
		config.RootCAs = cp
	}

	return &config, nil
}

// certPoolFromFile creates a new CertPool and appends the certificates from the given file.
func certPoolFromFile(filename string) (*x509.CertPool, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read CA file: %s", filename)
	}
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(b) {
		return nil, errors.Errorf("failed to append certificates from file: %s", filename)
	}

	return cp, nil
}

// getHelmChartAndRepoName returns chartName, repoURL as per the format requirred in helm install/upgrade config.
// For OCI charts, chartName needs to have whole URL path. e.g. oci://repo-url/chart-name
func getHelmChartAndRepoName(chartName, repoURL string) (string, string, error) {
	if registry.IsOCI(repoURL) {
		u, err := url.Parse(repoURL)
		if err != nil {
			return "", "", err
		}

		u.Path = path.Join(u.Path, chartName)
		chartName = u.String()
		repoURL = ""
	}

	return chartName, repoURL, nil
}

// UpgradeHelmReleaseIfChanged upgrades a Helm release. The boolean refers to if an upgrade was attempted.
func (c *HelmClient) UpgradeHelmReleaseIfChanged(ctx context.Context, restConfig *rest.Config, credentialsPath, caFilePath string, spec addonsv1alpha1.HelmReleaseProxySpec, existing *helmRelease.Release) (*helmRelease.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, restConfig)
	if err != nil {
		return nil, err
	}
	settings.RegistryConfig = credentialsPath

	enableClientCache := spec.Options.EnableClientCache
	log.V(2).Info("Creating upgrade registry client", "enableClientCache", enableClientCache, "credentialsPath", credentialsPath)

	registryClient, err := newDefaultRegistryClient(credentialsPath, enableClientCache, caFilePath, ptr.Deref(spec.TLSConfig, addonsv1alpha1.TLSConfig{}).InsecureSkipTLSVerify)
	if err != nil {
		return nil, err
	}

	actionConfig.RegistryClient = registryClient

	chartName, repoURL, err := getHelmChartAndRepoName(spec.ChartName, spec.RepoURL)
	if err != nil {
		return nil, err
	}

	upgradeClient := generateHelmUpgradeConfig(actionConfig, &spec.Options)
	upgradeClient.RepoURL = repoURL
	upgradeClient.Version = spec.Version
	upgradeClient.Namespace = spec.ReleaseNamespace

	log.V(2).Info("Locating chart...")
	cp, err := upgradeClient.ChartPathOptions.LocateChart(chartName, settings)
	if err != nil {
		return nil, err
	}
	log.V(2).Info("Located chart at path", "path", cp)

	log.V(2).Info("Writing values to file")
	filename, err := writeValuesToFile(ctx, spec)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := os.Remove(filename)
		if err != nil {
			log.Error(err, "Cannot remove file", "path", filename)
		}
	}()

	log.V(2).Info("Values written to file", "path", filename)
	content, err := os.ReadFile(filename)
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
	if chartRequested == nil {
		return nil, errors.Errorf("failed to load request chart %s", chartName)
	}

	shouldUpgrade, err := shouldUpgradeHelmRelease(ctx, *existing, chartRequested, vals)
	if err != nil {
		return nil, err
	}
	if !shouldUpgrade {
		log.V(2).Info(fmt.Sprintf("Release `%s` is up to date, no upgrade required, revision = %d", existing.Name, existing.Version))
		return existing, nil
	}

	log.V(1).Info("Upgrading with Helm", "release", spec.ReleaseName, "repo", spec.RepoURL)
	release, err := upgradeClient.RunWithContext(ctx, spec.ReleaseName, chartRequested, vals)

	return release, err
	// Should we force upgrade if it failed previously?
}

// writeValuesToFile writes the Helm values to a temporary file.
func writeValuesToFile(ctx context.Context, spec addonsv1alpha1.HelmReleaseProxySpec) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(2).Info("Writing values to file")
	valuesFile, err := os.CreateTemp("", spec.ChartName+"-"+spec.ReleaseName+"-*.yaml")
	if err != nil {
		return "", err
	}

	if _, err := valuesFile.Write([]byte(spec.Values)); err != nil {
		return "", err
	}

	return valuesFile.Name(), nil
}

// shouldUpgradeHelmRelease determines if a Helm release should be upgraded.
func shouldUpgradeHelmRelease(ctx context.Context, existing helmRelease.Release, chartRequested *chart.Chart, values map[string]interface{}) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	if existing.Chart == nil || existing.Chart.Metadata == nil {
		return false, errors.New("Failed to resolve chart version of existing release")
	}
	if existing.Chart.Metadata.Version != chartRequested.Metadata.Version {
		log.V(3).Info("Versions are different, upgrading")
		return true, nil
	}

	if existing.Info.Status == helmRelease.StatusFailed {
		log.Info("Release is in failed state, attempting upgrade to fix it")
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

// GetHelmRelease returns a Helm release if it exists.
func (c *HelmClient) GetHelmRelease(ctx context.Context, restConfig *rest.Config, spec addonsv1alpha1.HelmReleaseProxySpec) (*helmRelease.Release, error) {
	if spec.ReleaseName == "" {
		return nil, helmDriver.ErrReleaseNotFound
	}

	_, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, restConfig)
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

// ListHelmReleases lists all Helm releases in a namespace.
func (c *HelmClient) ListHelmReleases(ctx context.Context, restConfig *rest.Config, spec addonsv1alpha1.HelmReleaseProxySpec) ([]*helmRelease.Release, error) {
	_, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, restConfig)
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

// generateHelmUninstallConfig generates default helm uninstall config using helmOptions specified in HCP CR spec.
func generateHelmUninstallConfig(actionConfig *helmAction.Configuration, helmOptions *addonsv1alpha1.HelmOptions) *helmAction.Uninstall {
	uninstallClient := helmAction.NewUninstall(actionConfig)
	if helmOptions == nil {
		return uninstallClient
	}

	uninstallClient.DisableHooks = helmOptions.DisableHooks
	uninstallClient.Wait = helmOptions.Wait
	if helmOptions.Timeout != nil {
		uninstallClient.Timeout = helmOptions.Timeout.Duration
	}

	if helmOptions.Uninstall != nil {
		uninstallClient.KeepHistory = helmOptions.Uninstall.KeepHistory
		uninstallClient.Description = helmOptions.Uninstall.Description
	}

	return uninstallClient
}

// UninstallHelmRelease uninstalls a Helm release.
func (c *HelmClient) UninstallHelmRelease(ctx context.Context, restConfig *rest.Config, spec addonsv1alpha1.HelmReleaseProxySpec) (*helmRelease.UninstallReleaseResponse, error) {
	_, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, restConfig)
	if err != nil {
		return nil, err
	}

	uninstallClient := generateHelmUninstallConfig(actionConfig, &spec.Options)

	response, err := uninstallClient.Run(spec.ReleaseName)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// RollbackHelmRelease rolls back a Helm release.
func (c *HelmClient) RollbackHelmRelease(ctx context.Context, restConfig *rest.Config, spec addonsv1alpha1.HelmReleaseProxySpec) error {
	_, actionConfig, err := HelmInit(ctx, spec.ReleaseNamespace, restConfig)
	if err != nil {
		return err
	}

	rollbackClient := helmAction.NewRollback(actionConfig)

	return rollbackClient.Run(spec.ReleaseName)
}
