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
	"regexp"
	"strings"

	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1beta1 "cluster-api-addon-helm/api/v1beta1"
)

func ParseValues(ctx context.Context, c ctrlClient.Client, kubeconfigPath string, spec addonsv1beta1.HelmChartProxySpec, cluster *clusterv1.Cluster) ([]string, error) {
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
			substitution, err := LookUpSubstitution(ctx, c, cluster, innerField)
			if err != nil {
				return nil, err
			}
			substitutions[i] = substitution
		}
		substitutedValue := SubstituteValues(ctx, v, matches, substitutions)
		specValues = append(specValues, fmt.Sprintf("%s=%s", k, substitutedValue))
	}
	log.V(3).Info("Values", "values", specValues)

	return specValues, nil
}

func LookUpSubstitution(ctx context.Context, c ctrlClient.Client, cluster *clusterv1.Cluster, field string) (string, error) {
	log := ctrl.LoggerFrom(ctx)

	tokens := strings.Split(strings.TrimSpace(field), ".")
	if len(tokens) < 2 {
		return "", errors.Errorf("invalid field path %s, only kind identifier found", field)
	}
	kind := tokens[0]
	fieldPath := tokens[1:]
	switch kind {
	case "Cluster":
		fieldValue, err := GetClusterField(ctx, c, cluster, fieldPath)
		if err != nil {
			log.Error(err, "Failed to get cluster field", "fieldpath", fieldPath)
			return "", err
		}
		return fieldValue, nil
	default:
		return "", errors.Errorf("invalid field path %s, kind %s not supported", field, kind)
	}

}

func SubstituteValues(ctx context.Context, value string, matches [][]string, substitutions []string) string {
	for i, match := range matches {
		toReplace := match[0]
		value = strings.Replace(value, toReplace, substitutions[i], 1)
	}

	return value
}
