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
	"bytes"
	"context"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/external"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// initializeBuiltins initializes built-in templating objects for a given cluster.
// It retrieves the cluster, control plane, and infrastructure objects, converting them to unstructured maps for use in Helm chart value templates.
func initializeBuiltins(ctx context.Context, c ctrlClient.Client, cluster *clusterv1.Cluster) (map[string]interface{}, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Initializing built-in templating objects for cluster", "cluster", cluster.GetName())

	valueLookUp := make(map[string]interface{})

	// Transform cluster to unstructured
	clusterUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cluster)
	if err != nil {
		return nil, errors.Wrapf(err, "error converting cluster '%s' to unstructured", cluster.GetName())
	}

	valueLookUp["Cluster"] = clusterUnstructured

	if cluster.Spec.ControlPlaneRef.Name != "" && cluster.Spec.ControlPlaneRef.Kind != "" {
		objControlPlane, err := external.GetObjectFromContractVersionedRef(ctx, c, cluster.Spec.ControlPlaneRef, cluster.Namespace)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting ControlPlane object for cluster '%s'", cluster.GetName())
		}
		valueLookUp["ControlPlane"] = objControlPlane.Object
	}

	if cluster.Spec.InfrastructureRef.Name != "" && cluster.Spec.InfrastructureRef.Kind != "" {
		objInfraCluster, err := external.GetObjectFromContractVersionedRef(ctx, c, cluster.Spec.InfrastructureRef, cluster.Namespace)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting Infrastructure object for cluster '%s'", cluster.GetName())
		}

		valueLookUp["InfraCluster"] = objInfraCluster.Object
	}

	// TODO: would we want to add ControlPlaneMachineTemplate?

	return valueLookUp, nil
}

// ParseValues parses the values template and returns the expanded template. It attempts to populate a map of supported templating objects.
func ParseValues(ctx context.Context, c ctrlClient.Client, spec addonsv1alpha1.HelmChartProxySpec, cluster *clusterv1.Cluster) (string, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Rendering templating in values:", "values", spec.ValuesTemplate)

	valueLookUp, err := initializeBuiltins(ctx, c, cluster)
	if err != nil {
		return "", errors.Wrapf(err, "error initializing built-in templating objects for cluster '%s'", cluster.GetName())
	}

	tmpl, err := template.New(spec.ChartName + "-" + cluster.GetName()).
		Funcs(sprig.TxtFuncMap()).
		Parse(spec.ValuesTemplate)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer

	if err := tmpl.Execute(&buffer, valueLookUp); err != nil {
		return "", errors.Wrapf(err, "error executing template string '%s' on cluster '%s'", spec.ValuesTemplate, cluster.GetName())
	}
	expandedTemplate := buffer.String()
	log.V(2).Info("Expanded values to", "result", expandedTemplate)

	return expandedTemplate, nil
}
