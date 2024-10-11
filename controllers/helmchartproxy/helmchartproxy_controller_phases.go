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

package helmchartproxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"sigs.k8s.io/cluster-api-addon-provider-helm/internal"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// deleteOrphanedHelmReleaseProxies deletes any HelmReleaseProxy resources that belong to a Cluster that is not selected by its parent HelmChartProxy.
func (r *HelmChartProxyReconciler) deleteOrphanedHelmReleaseProxies(ctx context.Context, helmChartProxy *addonsv1alpha1.HelmChartProxy, clusters []clusterv1.Cluster, helmReleaseProxies []addonsv1alpha1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	releasesToDelete := getOrphanedHelmReleaseProxies(ctx, clusters, helmReleaseProxies)
	log.V(2).Info("Deleting orphaned releases")
	for i := range releasesToDelete {
		release := releasesToDelete[i]

		log.V(2).Info("Deleting release", "release", release)
		if err := r.deleteHelmReleaseProxy(ctx, &release); err != nil {
			conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition, addonsv1alpha1.HelmReleaseProxyDeletionFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return err
		}
	}

	return nil
}

// reconcileForCluster will create or update a HelmReleaseProxy for the given cluster.
func (r *HelmChartProxyReconciler) reconcileForCluster(ctx context.Context, helmChartProxy *addonsv1alpha1.HelmChartProxy, cluster clusterv1.Cluster) error {
	log := ctrl.LoggerFrom(ctx)

	// Don't reconcile if the Cluster or the helmChartProxy is paused.
	if annotations.IsPaused(&cluster, helmChartProxy) {
		log.V(2).Info("Reconciliation is paused for this Cluster", "cluster", cluster.Name)
		return nil
	}

	existingHelmReleaseProxy, err := r.getExistingHelmReleaseProxy(ctx, helmChartProxy, &cluster)
	if err != nil {
		// TODO: Should we set a condition here?
		return errors.Wrapf(err, "failed to get HelmReleaseProxy for cluster %s", cluster.Name)
	}
	// log.V(2).Info("Found existing HelmReleaseProxy", "cluster", cluster.Name, "release", existingHelmReleaseProxy.Name)

	if existingHelmReleaseProxy != nil && shouldReinstallHelmRelease(ctx, existingHelmReleaseProxy, helmChartProxy) {
		log.V(2).Info("Reinstalling Helm release by deleting and creating HelmReleaseProxy", "helmReleaseProxy", existingHelmReleaseProxy.Name)
		if err := r.deleteHelmReleaseProxy(ctx, existingHelmReleaseProxy); err != nil {
			conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition, addonsv1alpha1.HelmReleaseProxyDeletionFailedReason, clusterv1.ConditionSeverityError, err.Error())

			return err
		}

		// TODO: Add a check on requeue to make sure that the HelmReleaseProxy isn't still deleting
		log.V(2).Info("Successfully deleted HelmReleaseProxy on cluster, returning to requeue for reconcile", "cluster", cluster.Name)
		conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition, addonsv1alpha1.HelmReleaseProxyReinstallingReason, clusterv1.ConditionSeverityInfo, "HelmReleaseProxy on cluster '%s' successfully deleted, preparing to reinstall", cluster.Name)

		return nil // Try returning early so it will requeue
		// TODO: should we continue in the loop or just requeue?
	}

	values, err := internal.ParseValues(ctx, r.Client, helmChartProxy.Spec, &cluster)
	if err != nil {
		conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition, addonsv1alpha1.ValueParsingFailedReason, clusterv1.ConditionSeverityError, err.Error())

		return errors.Wrapf(err, "failed to parse values on cluster %s", cluster.Name)
	}

	log.V(2).Info("Values for cluster", "cluster", cluster.Name, "values", values)

	version := helmChartProxy.Spec.Version
	if helmChartProxy.Spec.VersionTemplate != "" {
		version, err = internal.ParseVersion(ctx, r.Client, helmChartProxy.Spec, &cluster)
		if err != nil {
			conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition, addonsv1alpha1.ValueParsingFailedReason, clusterv1.ConditionSeverityError, err.Error())

			return errors.Wrapf(err, "failed to parse version on cluster %s", cluster.Name)
		}
	}

	if err := r.createOrUpdateHelmReleaseProxy(ctx, existingHelmReleaseProxy, helmChartProxy, &cluster, values, version); err != nil {
		conditions.MarkFalse(helmChartProxy, addonsv1alpha1.HelmReleaseProxySpecsUpToDateCondition, addonsv1alpha1.HelmReleaseProxyCreationFailedReason, clusterv1.ConditionSeverityError, err.Error())

		return errors.Wrapf(err, "failed to create or update HelmReleaseProxy on cluster %s", cluster.Name)
	}

	return nil
}

// getExistingHelmReleaseProxy returns the HelmReleaseProxy for the given cluster if it exists.
func (r *HelmChartProxyReconciler) getExistingHelmReleaseProxy(ctx context.Context, helmChartProxy *addonsv1alpha1.HelmChartProxy, cluster *clusterv1.Cluster) (*addonsv1alpha1.HelmReleaseProxy, error) {
	log := ctrl.LoggerFrom(ctx)

	helmReleaseProxyList := &addonsv1alpha1.HelmReleaseProxyList{}

	listOpts := []client.ListOption{
		client.InNamespace(helmChartProxy.Namespace),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel:             cluster.Name,
			addonsv1alpha1.HelmChartProxyLabelName: helmChartProxy.Name,
		},
	}

	// TODO: Figure out if we want this search to be cross-namespaces.

	log.V(2).Info("Attempting to fetch existing HelmReleaseProxy with Cluster and HelmChartProxy labels", "cluster", cluster.Name, "helmChartProxy", helmChartProxy.Name)
	if err := r.Client.List(ctx, helmReleaseProxyList, listOpts...); err != nil {
		return nil, err
	}

	if helmReleaseProxyList.Items == nil || len(helmReleaseProxyList.Items) == 0 {
		log.V(2).Info("No HelmReleaseProxy found matching the cluster and HelmChartProxy", "cluster", cluster.Name, "helmChartProxy", helmChartProxy.Name)
		return nil, nil
	} else if len(helmReleaseProxyList.Items) > 1 {
		log.V(2).Info("Multiple HelmReleaseProxies found matching the cluster and HelmChartProxy", "cluster", cluster.Name, "helmChartProxy", helmChartProxy.Name)
		return nil, errors.Errorf("multiple HelmReleaseProxies found matching the cluster and HelmChartProxy")
	}

	log.V(2).Info("Found existing matching HelmReleaseProxy", "cluster", cluster.Name, "helmChartProxy", helmChartProxy.Name)

	return &helmReleaseProxyList.Items[0], nil
}

// createOrUpdateHelmReleaseProxy creates or updates the HelmReleaseProxy for the given cluster.
func (r *HelmChartProxyReconciler) createOrUpdateHelmReleaseProxy(ctx context.Context, existing *addonsv1alpha1.HelmReleaseProxy, helmChartProxy *addonsv1alpha1.HelmChartProxy, cluster *clusterv1.Cluster, parsedValues, parsedVersion string) error {
	log := ctrl.LoggerFrom(ctx)
	helmReleaseProxy := constructHelmReleaseProxy(existing, helmChartProxy, parsedValues, parsedVersion, cluster)
	if helmReleaseProxy == nil {
		log.V(2).Info("HelmReleaseProxy is up to date, nothing to do", "helmReleaseProxy", existing.Name, "cluster", cluster.Name)
		return nil
	}
	if existing == nil {
		if err := r.Client.Create(ctx, helmReleaseProxy); err != nil {
			return errors.Wrapf(err, "failed to create HelmReleaseProxy '%s' for cluster: %s/%s", helmReleaseProxy.Name, cluster.Namespace, cluster.Name)
		}
	} else {
		// TODO: should this use patchHelmReleaseProxy() instead of Update() in case there's a race condition?
		if err := r.Client.Update(ctx, helmReleaseProxy); err != nil {
			return errors.Wrapf(err, "failed to update HelmReleaseProxy '%s' for cluster: %s/%s", helmReleaseProxy.Name, cluster.Namespace, cluster.Name)
		}
	}

	return nil
}

// deleteHelmReleaseProxy deletes the HelmReleaseProxy for the given cluster.
func (r *HelmChartProxyReconciler) deleteHelmReleaseProxy(ctx context.Context, helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy) error {
	log := ctrl.LoggerFrom(ctx)

	if err := r.Client.Delete(ctx, helmReleaseProxy); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("HelmReleaseProxy already deleted, nothing to do", "helmReleaseProxy", helmReleaseProxy.Name)
			return nil
		}

		return errors.Wrapf(err, "failed to delete helmReleaseProxy: %s", helmReleaseProxy.Name)
	}

	return nil
}

// constructHelmReleaseProxy constructs a new HelmReleaseProxy for the given Cluster or updates the existing HelmReleaseProxy if needed.
// If no update is needed, this returns nil. Note that this does not check if we need to reinstall the HelmReleaseProxy, i.e. immutable fields changed.
func constructHelmReleaseProxy(existing *addonsv1alpha1.HelmReleaseProxy, helmChartProxy *addonsv1alpha1.HelmChartProxy, parsedValues, parsedVersion string, cluster *clusterv1.Cluster) *addonsv1alpha1.HelmReleaseProxy {
	helmReleaseProxy := &addonsv1alpha1.HelmReleaseProxy{}

	// If it's not set, then use the as-is version to make tests happy without breaking the logic.
	if parsedVersion == "" {
		parsedVersion = helmChartProxy.Spec.Version
	}

	if existing == nil {
		helmReleaseProxy.GenerateName = fmt.Sprintf("%s-%s-", helmChartProxy.Spec.ChartName, cluster.Name)
		helmReleaseProxy.Namespace = helmChartProxy.Namespace
		helmReleaseProxy.OwnerReferences = util.EnsureOwnerRef(helmReleaseProxy.OwnerReferences, *metav1.NewControllerRef(helmChartProxy, helmChartProxy.GroupVersionKind()))

		newLabels := map[string]string{}
		newLabels[clusterv1.ClusterNameLabel] = cluster.Name
		newLabels[addonsv1alpha1.HelmChartProxyLabelName] = helmChartProxy.Name
		helmReleaseProxy.Labels = newLabels

		helmReleaseProxy.Spec.ClusterRef = corev1.ObjectReference{
			Kind:       cluster.Kind,
			APIVersion: cluster.APIVersion,
			Name:       cluster.Name,
			Namespace:  cluster.Namespace,
		}

		helmReleaseProxy.Spec.ReleaseName = helmChartProxy.Spec.ReleaseName
		helmReleaseProxy.Spec.ChartName = helmChartProxy.Spec.ChartName
		helmReleaseProxy.Spec.RepoURL = helmChartProxy.Spec.RepoURL
		helmReleaseProxy.Spec.ReleaseNamespace = helmChartProxy.Spec.ReleaseNamespace

		// helmChartProxy.ObjectMeta.SetAnnotations(helmReleaseProxy.Annotations)
	} else {
		helmReleaseProxy = existing
		changed := false
		if existing.Spec.Version != helmChartProxy.Spec.Version {
			changed = true
		}
		if !cmp.Equal(existing.Spec.Values, parsedValues) {
			changed = true
		}
		if !cmp.Equal(existing.Spec.Version, parsedVersion) {
			changed = true
		}

		if !changed {
			return nil
		}
	}

	helmReleaseProxy.Spec.Version = strings.TrimSpace(parsedVersion)
	helmReleaseProxy.Spec.Values = parsedValues
	helmReleaseProxy.Spec.Options = helmChartProxy.Spec.Options
	helmReleaseProxy.Spec.Credentials = helmChartProxy.Spec.Credentials

	if helmReleaseProxy.Spec.Credentials != nil {
		// If the namespace is not set, set it to the namespace of the HelmChartProxy
		if helmReleaseProxy.Spec.Credentials.Secret.Namespace == "" {
			helmReleaseProxy.Spec.Credentials.Secret.Namespace = helmChartProxy.Namespace
		}

		// If the key is not set, set it to the default key
		if helmReleaseProxy.Spec.Credentials.Key == "" {
			helmReleaseProxy.Spec.Credentials.Key = addonsv1alpha1.DefaultOCIKey
		}
	}

	helmReleaseProxy.Spec.TLSConfig = helmChartProxy.Spec.TLSConfig

	if helmReleaseProxy.Spec.TLSConfig != nil && helmReleaseProxy.Spec.TLSConfig.CASecretRef != nil {
		// If the namespace is not set, set it to the namespace of the HelmChartProxy
		if helmReleaseProxy.Spec.TLSConfig.CASecretRef.Namespace == "" {
			helmReleaseProxy.Spec.TLSConfig.CASecretRef.Namespace = helmChartProxy.Namespace
		}
	}

	return helmReleaseProxy
}

// shouldReinstallHelmRelease returns true if the HelmReleaseProxy needs to be reinstalled. This is the case if any of the immutable fields changed.
func shouldReinstallHelmRelease(ctx context.Context, existing *addonsv1alpha1.HelmReleaseProxy, helmChartProxy *addonsv1alpha1.HelmChartProxy) bool {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Checking if HelmReleaseProxy needs to be reinstalled by by checking if immutable fields changed", "helmReleaseProxy", existing.Name)

	annotations := existing.GetAnnotations()
	result, ok := annotations[addonsv1alpha1.IsReleaseNameGeneratedAnnotation]

	isReleaseNameGenerated := ok && result == "true"
	switch {
	case existing.Spec.ChartName != helmChartProxy.Spec.ChartName:
		log.V(2).Info("ChartName changed", "existing", existing.Spec.ChartName, "helmChartProxy", helmChartProxy.Spec.ChartName)
		return true
	case existing.Spec.RepoURL != helmChartProxy.Spec.RepoURL:
		log.V(2).Info("RepoURL changed", "existing", existing.Spec.RepoURL, "helmChartProxy", helmChartProxy.Spec.RepoURL)
		return true
	case isReleaseNameGenerated && helmChartProxy.Spec.ReleaseName != "":
		log.V(2).Info("Generated ReleaseName changed", "existing", existing.Spec.ReleaseName, "helmChartProxy", helmChartProxy.Spec.ReleaseName)
		return true
	case !isReleaseNameGenerated && existing.Spec.ReleaseName != helmChartProxy.Spec.ReleaseName:
		log.V(2).Info("Non-generated ReleaseName changed", "existing", existing.Spec.ReleaseName, "helmChartProxy", helmChartProxy.Spec.ReleaseName)
		return true
	case existing.Spec.ReleaseNamespace != helmChartProxy.Spec.ReleaseNamespace:
		log.V(2).Info("ReleaseNamespace changed", "existing", existing.Spec.ReleaseNamespace, "helmChartProxy", helmChartProxy.Spec.ReleaseNamespace)
		return true
	}

	return false
}

// getOrphanedHelmReleaseProxies returns a list of HelmReleaseProxies that are not associated with any of the selected Clusters for a given HelmChartProxy.
func getOrphanedHelmReleaseProxies(ctx context.Context, clusters []clusterv1.Cluster, helmReleaseProxies []addonsv1alpha1.HelmReleaseProxy) []addonsv1alpha1.HelmReleaseProxy {
	log := ctrl.LoggerFrom(ctx)
	log.V(2).Info("Getting HelmReleaseProxies to delete")

	selectedClusters := map[string]struct{}{}
	for _, cluster := range clusters {
		key := cluster.GetNamespace() + "/" + cluster.GetName()
		selectedClusters[key] = struct{}{}
	}
	log.V(2).Info("Selected clusters", "clusters", selectedClusters)

	releasesToDelete := []addonsv1alpha1.HelmReleaseProxy{}
	for _, helmReleaseProxy := range helmReleaseProxies {
		clusterRef := helmReleaseProxy.Spec.ClusterRef
		key := clusterRef.Namespace + "/" + clusterRef.Name
		if _, ok := selectedClusters[key]; !ok {
			releasesToDelete = append(releasesToDelete, helmReleaseProxy)
		}
	}

	names := make([]string, len(releasesToDelete))
	for i, release := range releasesToDelete {
		names[i] = release.Name
	}
	log.V(2).Info("Releases to delete", "releases", names)

	return releasesToDelete
}
