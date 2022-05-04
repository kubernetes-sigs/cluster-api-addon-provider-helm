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
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"
)

func initializeBuiltins(ctx context.Context, c ctrlClient.Client, cluster *clusterv1.Cluster) (*BuiltinTypes, error) {
	kubeadmControlPlane := &kcpv1.KubeadmControlPlane{}
	key := types.NamespacedName{
		Name:      cluster.Spec.ControlPlaneRef.Name,
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
	}
	err := c.Get(ctx, key, kubeadmControlPlane)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get kubeadm control plane %s", key)
	}

	return &BuiltinTypes{
		Cluster:      cluster,
		ControlPlane: kubeadmControlPlane,
	}, nil
}

type BuiltinTypes struct {
	Cluster      *clusterv1.Cluster
	ControlPlane *kcpv1.KubeadmControlPlane
}

func ParseValues(ctx context.Context, c ctrlClient.Client, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec, cluster *clusterv1.Cluster) ([]string, error) {
	log := ctrl.LoggerFrom(ctx)
	specValues := make([]string, len(spec.Values))
	for k, v := range spec.Values {
		builtin, err := initializeBuiltins(ctx, c, cluster)
		if err != nil {
			return nil, err
		}

		tmpl, err := template.New(cluster.GetName() + "-" + k).
			// Use this function to dereference *int32 for comparisons
			// Funcs(template.FuncMap{
			// 	"DerefInt32": func(i *int32) int32 { return *i },
			// }).
			Parse(v)
		if err != nil {
			return nil, err
		}
		var buffer bytes.Buffer

		if err := tmpl.Execute(&buffer, builtin); err != nil {
			return nil, errors.Wrapf(err, "error executing template string '%s' on cluster '%s'", v, cluster.GetName())
		}
		expandedTemplate := buffer.String()
		log.V(2).Info("Expanded template", "template", v, "result", expandedTemplate)
		specValues = append(specValues, fmt.Sprintf("%s=%s", k, expandedTemplate))
	}
	log.V(3).Info("Values", "values", specValues)

	return specValues, nil
}
