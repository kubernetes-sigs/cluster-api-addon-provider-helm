//go:build e2e
// +build e2e

/*
Copyright 2024 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

var nginxValues = `controller:
  name: "{{ .ControlPlane.metadata.name }}-nginx"
  nginxStatus:
    allowCidrs: {{ index .Cluster.spec.clusterNetwork.pods.cidrBlocks 0 }}`

var newNginxValues = `controller:
  name: "{{ .Cluster.metadata.name }}-nginx"
  nginxStatus:
    allowCidrs: 127.0.0.1,::1,{{ index .Cluster.spec.clusterNetwork.pods.cidrBlocks 0 }}`

var _ = Describe("Workload cluster creation", func() {
	var (
		ctx               = context.Background()
		specName          = "create-workload-cluster"
		namespace         *corev1.Namespace
		cancelWatches     context.CancelFunc
		result            *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName       string
		clusterNamePrefix string
		additionalCleanup func()
		specTimes         = map[string]time.Time{}
	)

	BeforeEach(func() {
		logCheckpoint(specTimes)

		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(e2eConfig).NotTo(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0o755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersion))

		// CLUSTER_NAME and CLUSTER_NAMESPACE allows for testing existing clusters.
		// If CLUSTER_NAMESPACE is set, don't generate a new prefix. Otherwise,
		// the correct namespace won't be found and a new cluster will be created.
		clusterNameSpace := os.Getenv("CLUSTER_NAMESPACE")
		if clusterNameSpace == "" {
			clusterNamePrefix = fmt.Sprintf("caaph-e2e-%s", util.RandomString(6))
		} else {
			clusterNamePrefix = clusterNameSpace
		}

		// Set up a Namespace where to host objects for this spec and create a watcher for the namespace events.
		var err error
		namespace, cancelWatches, err = setupSpecNamespace(ctx, clusterNamePrefix, bootstrapClusterProxy, artifactFolder)
		Expect(err).NotTo(HaveOccurred())

		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		additionalCleanup = nil
	})

	AfterEach(func() {
		if result.Cluster == nil {
			// this means the cluster failed to come up. We make an attempt to find the cluster to be able to fetch logs for the failed bootstrapping.
			_ = bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{Name: clusterName, Namespace: namespace.Name}, result.Cluster)
		}

		CheckTestBeforeCleanup()

		cleanInput := cleanupInput{
			SpecName:          specName,
			Cluster:           result.Cluster,
			ClusterProxy:      bootstrapClusterProxy,
			Namespace:         namespace,
			CancelWatches:     cancelWatches,
			IntervalsGetter:   e2eConfig.GetIntervals,
			SkipCleanup:       skipCleanup,
			SkipLogCollection: skipLogCollection,
			AdditionalCleanup: additionalCleanup,
			ArtifactFolder:    artifactFolder,
		}
		dumpSpecResourcesAndCleanup(ctx, cleanInput)

		logCheckpoint(specTimes)
	})

	Context("Creating workload cluster [REQUIRED]", func() {
		It("With default template to install, upgrade, and uninstall nginx Helm chart", func() {
			clusterName = fmt.Sprintf("%s-%s", specName, util.RandomString(6))
			clusterctl.ApplyClusterTemplateAndWait(ctx, createApplyClusterTemplateInput(
				specName,
				withNamespace(namespace.Name),
				withClusterName(clusterName),
				withControlPlaneMachineCount(1),
				withWorkerMachineCount(1),
				withControlPlaneWaiters(clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				}),
			), result)

			hcp := &addonsv1alpha1.HelmChartProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-ingress",
					Namespace: namespace.Name,
				},
				Spec: addonsv1alpha1.HelmChartProxySpec{
					ClusterSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nginxIngress": "enabled",
						},
					},
					ReleaseName:      "nginx-ingress",
					ReleaseNamespace: "nginx-namespace",
					ChartName:        "nginx-ingress",
					RepoURL:          "https://helm.nginx.com/stable",
					ValuesTemplate:   nginxValues,
				},
			}

			// Create new Helm chart
			By("Creating new HelmChartProxy to install nginx", func() {
				HelmInstallSpec(ctx, func() HelmInstallInput {
					return HelmInstallInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						HelmChartProxy:        hcp,
					}
				})
			})

			// Update existing Helm chart
			By("Updating nginx HelmChartProxy valuesTemplate", func() {
				hcp.Spec.ValuesTemplate = newNginxValues
				HelmUpgradeSpec(ctx, func() HelmUpgradeInput {
					return HelmUpgradeInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						HelmChartProxy:        hcp,
						ExpectedRevision:      2,
					}
				})
			})

			// Force reinstall of existing Helm chart by changing the release namespace
			By("Updating HelmChartProxy release namespace", func() {
				hcp.Spec.ReleaseNamespace = "new-nginx-namespace"
				HelmUpgradeSpec(ctx, func() HelmUpgradeInput {
					return HelmUpgradeInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						HelmChartProxy:        hcp,
						ExpectedRevision:      1,
					}
				})
			})

			// Force reinstall of existing Helm chart by changing the release name
			By("Updating HelmChartProxy release name", func() {
				hcp.Spec.ReleaseName = "new-nginx-name"
				HelmUpgradeSpec(ctx, func() HelmUpgradeInput {
					return HelmUpgradeInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						HelmChartProxy:        hcp,
						ExpectedRevision:      1,
					}
				})
			})

			// Uninstall Helm chart by removing the label selector from the Cluster.
			By("Uninstalling Helm chart from cluster", func() {
				HelmUninstallSpec(ctx, func() HelmUninstallInput {
					return HelmUninstallInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						HelmChartProxy:        hcp,
					}
				})
			})
		})
	})
})

type cleanupInput struct {
	SpecName          string
	ClusterProxy      framework.ClusterProxy
	ArtifactFolder    string
	Namespace         *corev1.Namespace
	CancelWatches     context.CancelFunc
	Cluster           *clusterv1.Cluster
	IntervalsGetter   func(spec, key string) []interface{}
	SkipCleanup       bool
	SkipLogCollection bool
	AdditionalCleanup func()
}

func dumpSpecResourcesAndCleanup(ctx context.Context, input cleanupInput) {
	defer func() {
		input.CancelWatches()
	}()

	Logf("Dumping all the Cluster API resources in the %q namespace", input.Namespace.Name)
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    input.ClusterProxy.GetClient(),
		Namespace: input.Namespace.Name,
		LogPath:   filepath.Join(input.ArtifactFolder, "clusters", input.ClusterProxy.GetName(), "resources"),
	})

	if input.Cluster == nil {
		By("Unable to dump workload cluster logs as the cluster is nil")
	} else if !input.SkipLogCollection {
		Byf("Dumping logs from the %q workload cluster", input.Cluster.Name)
		input.ClusterProxy.CollectWorkloadClusterLogs(ctx, input.Cluster.Namespace, input.Cluster.Name, filepath.Join(input.ArtifactFolder, "clusters", input.Cluster.Name))
	}

	if input.SkipCleanup {
		return
	}

	Logf("Deleting all clusters in the %s namespace", input.Namespace.Name)
	// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
	// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
	// instead of DeleteClusterAndWait
	deleteTimeoutConfig := "wait-delete-cluster"
	framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
		Client:    input.ClusterProxy.GetClient(),
		Namespace: input.Namespace.Name,
	}, input.IntervalsGetter(input.SpecName, deleteTimeoutConfig)...)

	Logf("Deleting namespace used for hosting the %q test spec", input.SpecName)
	framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
		Deleter: input.ClusterProxy.GetClient(),
		Name:    input.Namespace.Name,
	})

	if input.AdditionalCleanup != nil {
		Logf("Running additional cleanup for the %q test spec", input.SpecName)
		input.AdditionalCleanup()
	}
}
