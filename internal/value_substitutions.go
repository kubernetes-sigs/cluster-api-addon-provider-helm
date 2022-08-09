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

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1beta1 "cluster-api-addon-provider-helm/api/v1beta1"
)

func initializeBuiltins(ctx context.Context, c ctrlClient.Client, spec addonsv1beta1.HelmChartProxySpec, cluster *clusterv1.Cluster) (*BuiltinTypes, error) {
	kubeadmControlPlane := &kcpv1.KubeadmControlPlane{}
	key := types.NamespacedName{
		Name:      cluster.Spec.ControlPlaneRef.Name,
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
	}
	err := c.Get(ctx, key, kubeadmControlPlane)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get kubeadm control plane %s", key)
	}

	builtInTypes := BuiltinTypes{
		Cluster:            cluster,
		ControlPlane:       kubeadmControlPlane,
		MachineDeployments: map[string]clusterv1.MachineDeployment{},
		MachineSets:        map[string]clusterv1.MachineSet{},
		Machines:           map[string]clusterv1.Machine{},
	}

	// Comment out the custom selectors until we can define a use case
	// for key, selectorSpec := range spec.CustomSelectors {
	// 	labels := selectorSpec.Selector.MatchLabels
	// 	labels[clusterv1.ClusterLabelName] = cluster.Name
	// 	switch selectorSpec.Kind {
	// 	case "MachineDeployment":
	// 		machineDeployments := &clusterv1.MachineDeploymentList{}
	// 		if err := c.List(ctx, machineDeployments, ctrlClient.MatchingLabels(labels)); err != nil {
	// 			return nil, errors.Wrapf(err, "failed to list machine deployments with selector %v", selectorSpec.Selector.MatchLabels)
	// 		}
	// 		if len(machineDeployments.Items) == 0 {
	// 			return nil, errors.Errorf("no machine deployments found with selector %v", selectorSpec.Selector.MatchLabels)
	// 		}
	// 		if len(machineDeployments.Items) > 1 {
	// 			return nil, errors.Errorf("multiple machine deployments found with selector %v", selectorSpec.Selector.MatchLabels)
	// 		}
	// 		builtInTypes.MachineDeployments[key] = machineDeployments.Items[0]
	// 		break
	// 	case "MachineSet":
	// 		machineSets := &clusterv1.MachineSetList{}
	// 		if err := c.List(ctx, machineSets, ctrlClient.MatchingLabels(labels)); err != nil {
	// 			return nil, errors.Wrapf(err, "failed to list machine sets with selector %v", selectorSpec.Selector.MatchLabels)
	// 		}
	// 		if len(machineSets.Items) == 0 {
	// 			return nil, errors.Errorf("no machine sets found with selector %v", selectorSpec.Selector.MatchLabels)
	// 		}
	// 		if len(machineSets.Items) > 1 {
	// 			return nil, errors.Errorf("multiple machine sets found with selector %v", selectorSpec.Selector.MatchLabels)
	// 		}
	// 		builtInTypes.MachineSets[key] = machineSets.Items[0]
	// 		break
	// 	case "Machine":
	// 		machines := &clusterv1.MachineList{}
	// 		if err := c.List(ctx, machines, ctrlClient.MatchingLabels(labels)); err != nil {
	// 			return nil, errors.Wrapf(err, "failed to list machines with selector %v", selectorSpec.Selector.MatchLabels)
	// 		}
	// 		if len(machines.Items) == 0 {
	// 			return nil, errors.Errorf("no machines found with selector %v", selectorSpec.Selector.MatchLabels)
	// 		}
	// 		if len(machines.Items) > 1 {
	// 			return nil, errors.Errorf("multiple machines found with selector %v", selectorSpec.Selector.MatchLabels)
	// 		}
	// 		builtInTypes.Machines[key] = machines.Items[0]
	// 		break
	// 	default:
	// 		return nil, errors.Errorf("unsupported selector kind %s", selectorSpec.Kind)
	// 	}
	// }

	return &builtInTypes, nil
}

type BuiltinTypes struct {
	Cluster            *clusterv1.Cluster
	ControlPlane       *kcpv1.KubeadmControlPlane
	MachineDeployments map[string]clusterv1.MachineDeployment
	MachineSets        map[string]clusterv1.MachineSet
	Machines           map[string]clusterv1.Machine
}

func ParseValues(ctx context.Context, c ctrlClient.Client, spec addonsv1beta1.HelmChartProxySpec, cluster *clusterv1.Cluster) (string, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(2).Info("Rendering templating in values:", "values", spec.Values)
	builtin, err := initializeBuiltins(ctx, c, spec, cluster)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(spec.ChartName + "-" + cluster.GetName()).
		Funcs(sprig.TxtFuncMap()).
		Parse(spec.Values)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer

	if err := tmpl.Execute(&buffer, builtin); err != nil {
		return "", errors.Wrapf(err, "error executing template string '%s' on cluster '%s'", spec.Values, cluster.GetName())
	}
	expandedTemplate := buffer.String()
	log.V(2).Info("Expanded values to", "result", expandedTemplate)

	return expandedTemplate, nil
}

func ValueMapToArray(valueMap map[string]string) []string {
	valueArray := make([]string, 0, len(valueMap))
	for k, v := range valueMap {
		valueArray = append(valueArray, fmt.Sprintf("%s=%s", k, v))
	}

	return valueArray
}
