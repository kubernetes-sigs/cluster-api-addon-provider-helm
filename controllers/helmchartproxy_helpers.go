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
	"regexp"
	"strings"

	"github.com/pkg/errors"
	helmAction "helm.sh/helm/v3/pkg/action"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	"sigs.k8s.io/cluster-api/controllers/external"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
)

func writeClusterKubeconfigToFile(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	c, err := client.New("")
	if err != nil {
		return "", err
	}

	options := client.GetKubeconfigOptions{
		Kubeconfig: client.Kubeconfig{},
		// Kubeconfig:          client.Kubeconfig{Path: gk.kubeconfig, Context: gk.kubeconfigContext},
		WorkloadClusterName: cluster.Name,
		Namespace:           cluster.Namespace,
	}

	log.V(4).Info("Getting kubeconfig for cluster", "cluster", cluster.Name)
	kubeconfig, err := c.GetKubeconfig(options)
	if err != nil {
		return "", err
	}
	log.V(4).Info("cluster", "cluster", cluster.Name, "kubeconfig is:", kubeconfig)

	path := "tmp"
	filePath := path + "/" + cluster.Name
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			return "", errors.Wrapf(err, "failed to create directory %s", path)
		}
	}
	f, err := os.Create(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create file %s", filePath)
	}

	log.V(4).Info("Writing kubeconfig to file", "cluster", cluster.Name)
	_, err = f.WriteString(kubeconfig)
	if err != nil {
		f.Close()
		return "", errors.Wrapf(err, "failed to close kubeconfig file")
	}
	err = f.Close()
	if err != nil {
		return "", errors.Wrapf(err, "failed to close kubeconfig file")
	}

	log.V(4).Info("Path is", "path", path)
	return filePath, nil
}

func parseValues(ctx context.Context, c ctrlClient.Client, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec, cluster *clusterv1.Cluster) ([]string, error) {
	log := ctrl.LoggerFrom(ctx)
	specValues := make([]string, len(spec.Values))
	for k, v := range spec.Values {
		r := regexp.MustCompile(`{{(.+?)}}`)
		matches := r.FindAllStringSubmatch(v, -1)
		substitutions := make([]string, len(matches))
		for i, match := range matches {
			log.V(3).Info("Found match", "match", match)
			// For {{ cluster.metadata.name }}, match[0] is {{ cluster.metadata.name }} and match[1] is ` cluster.metadata.name ` (including whitespace)
			innerField := match[1]
			substitution, err := lookUpSubstitution(ctx, c, cluster, innerField)
			if err != nil {
				return nil, err
			}
			substitutions[i] = substitution
		}
		substitutedValue := substituteValues(ctx, v, matches, substitutions)
		specValues = append(specValues, fmt.Sprintf("%s=%s", k, substitutedValue))
	}

	return specValues, nil
}

func lookUpSubstitution(ctx context.Context, c ctrlClient.Client, cluster *clusterv1.Cluster, field string) (string, error) {
	log := ctrl.LoggerFrom(ctx)

	tokens := strings.Split(strings.TrimSpace(field), ".")
	if len(tokens) < 2 {
		return "", errors.Errorf("invalid field path %s, only kind identifier found", field)
	}
	kind := tokens[0]
	fieldPath := tokens[1:]
	switch kind {
	case "Cluster":
		fieldValue, err := getClusterField(ctx, c, cluster, fieldPath)
		if err != nil {
			log.Error(err, "Failed to get cluster field", "fieldpath", fieldPath)
			return "", err
		}
		return fieldValue, nil
	default:
		return "", errors.Errorf("invalid field path %s, kind %s not supported", field, kind)
	}

}

func substituteValues(ctx context.Context, value string, matches [][]string, substitutions []string) string {
	for i, match := range matches {
		toReplace := match[0]
		value = strings.Replace(value, toReplace, substitutions[i], 1)
	}

	return value
}

func getCustomResource(ctx context.Context, c ctrlClient.Client, kind string, apiVersion string, namespace string, name string) (*unstructured.Unstructured, error) {
	objectRef := corev1.ObjectReference{
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
		APIVersion: apiVersion,
	}
	object, err := external.Get(context.TODO(), c, &objectRef, namespace)
	if err != nil {
		return nil, nil
	}

	return object, nil
}

func getClusterField(ctx context.Context, c ctrlClient.Client, cluster *clusterv1.Cluster, fields []string) (string, error) {
	log := ctrl.LoggerFrom(ctx)

	object, err := getCustomResource(ctx, c, cluster.Kind, cluster.APIVersion, cluster.Namespace, cluster.Name)
	if err != nil {
		return "", err
	}
	objectMap := object.UnstructuredContent()
	field, found, err := unstructured.NestedString(objectMap, fields...)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get cluster name from cluster object")
	}
	if !found {
		return "", errors.New("failed to get cluster name from cluster object")
	}
	log.V(2).Info("Resolved cluster field to", "field", fields, "value", field)

	return field, nil
}

func helmInit(ctx context.Context, kubeconfigPath string) (*helmCli.EnvSettings, *helmAction.Configuration, error) {
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

func installHelmRelease(ctx context.Context, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec, parsedValues []string) (*release.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := helmInit(ctx, kubeconfigPath)
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
	// specValues := make([]string, len(spec.Values))
	// for k, v := range spec.Values {
	// 	specValues = append(specValues, fmt.Sprintf("%s=%s", k, v))
	// }
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
func upgradeHelmRelease(ctx context.Context, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec, parsedValues []string) (*release.Release, error) {
	log := ctrl.LoggerFrom(ctx)

	settings, actionConfig, err := helmInit(ctx, kubeconfigPath)
	if err != nil {
		return nil, err
	}
	upgrader := helmAction.NewUpgrade(actionConfig)
	upgrader.RepoURL = spec.RepoURL
	upgrader.Version = spec.Version
	upgrader.Namespace = "default"
	cp, err := upgrader.ChartPathOptions.LocateChart(spec.ChartName, settings)
	log.V(2).Info("Located chart at path", "path", cp)
	if err != nil {
		return nil, err
	}
	p := helmGetter.All(settings)
	// specValues := make([]string, len(spec.Values))
	// for k, v := range spec.Values {
	// 	specValues = append(specValues, fmt.Sprintf("%s=%s", k, v))
	// }
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
	log.V(2).Info("Upgrading with Helm...")
	release, err := upgrader.RunWithContext(ctx, spec.ReleaseName, chartRequested, vals)
	if err != nil {
		return nil, err
	}

	return release, nil
}

func getHelmRelease(ctx context.Context, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec) (*release.Release, error) {
	_, actionConfig, err := helmInit(ctx, kubeconfigPath)
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

func listHelmReleases(ctx context.Context, kubeconfigPath string) ([]*release.Release, error) {
	_, actionConfig, err := helmInit(ctx, kubeconfigPath)
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

func uninstallHelmRelease(ctx context.Context, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec) (*release.UninstallReleaseResponse, error) {
	_, actionConfig, err := helmInit(ctx, kubeconfigPath)
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

// func shouldUpdateHelmRelease(existing *release.Release, spec *addonsv1beta1.HelmChartProxySpec) bool {
// 	if existing == nil {
// 		return false
// 	}

// 	// if spec.RepoURL != existing.Chart.Repo {
// 	// 	return true
// 	// }
// 	return true
// }
