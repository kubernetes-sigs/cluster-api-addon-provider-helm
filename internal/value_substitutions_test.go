/*
Copyright 2026 The Kubernetes Authors.

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
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestSecureTxtFuncMap(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	funcMap := secureTxtFuncMap()

	for _, name := range unsafeTemplateFuncs {
		g.Expect(funcMap).NotTo(HaveKey(name), "expected unsafe function %q to be removed", name)
	}

	// Commonly used helpers must still be available so existing templates keep working.
	for _, name := range []string{"upper", "lower", "default", "now", "b64enc"} {
		g.Expect(funcMap).To(HaveKey(name), "expected safe function %q to be present", name)
	}
}

func TestParseValuesRejectsUnsafeTemplateFunctions(t *testing.T) {
	t.Setenv("CAAPH_TEMPLATE_SECRET", "not-for-templates")

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
	}
	testCases := []struct {
		name           string
		valuesTemplate string
	}{
		{name: "env", valuesTemplate: `{{ env "CAAPH_TEMPLATE_SECRET" }}`},
		{name: "expandenv", valuesTemplate: `{{ expandenv "$CAAPH_TEMPLATE_SECRET" }}`},
		{name: "getHostByName", valuesTemplate: `{{ getHostByName "example.com" }}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			spec := addonsv1alpha1.HelmChartProxySpec{
				ChartName:      "test-chart",
				ValuesTemplate: tc.valuesTemplate,
			}

			_, err := ParseValues(context.Background(), nil, spec, cluster)
			g.Expect(err).To(HaveOccurred())
		})
	}
}
