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
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	helmAction "helm.sh/helm/v3/pkg/action"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmRelease "helm.sh/helm/v3/pkg/release"
	helmDriver "helm.sh/helm/v3/pkg/storage/driver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/klog/v2"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	sshPort                               = "22"
	deleteOperationTimeout                = 20 * time.Minute
	retryableOperationTimeout             = 30 * time.Second
	retryableDeleteOperationTimeout       = 3 * time.Minute
	retryableOperationSleepBetweenRetries = 3 * time.Second
	helmInstallTimeout                    = 3 * time.Minute
	sshConnectionTimeout                  = 30 * time.Second
)

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

// deploymentsClientAdapter adapts a Deployment to work with WaitForDeploymentsAvailable.
type deploymentsClientAdapter struct {
	client typedappsv1.DeploymentInterface
}

// Get fetches the deployment named by the key and updates the provided object.
func (c deploymentsClientAdapter) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	deployment, err := c.client.Get(ctx, key.Name, metav1.GetOptions{})
	if deployObj, ok := obj.(*appsv1.Deployment); ok {
		deployment.DeepCopyInto(deployObj)
	}
	return err
}

// WaitForDeploymentsAvailableInput is the input for WaitForDeploymentsAvailable.
type WaitForDeploymentsAvailableInput struct {
	Getter     framework.Getter
	Deployment *appsv1.Deployment
	Clientset  *kubernetes.Clientset
}

// WaitForDeploymentsAvailable waits until the Deployment has status.Available = True, that signals that
// all the desired replicas are in place.
// This can be used to check if Cluster API controllers installed in the management cluster are working.
func WaitForDeploymentsAvailable(ctx context.Context, input WaitForDeploymentsAvailableInput, intervals ...interface{}) {
	start := time.Now()
	namespace, name := input.Deployment.GetNamespace(), input.Deployment.GetName()
	Byf("waiting for deployment %s/%s to be available", namespace, name)
	Log("starting to wait for deployment to become available")
	Eventually(func() bool {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		if err := input.Getter.Get(ctx, key, input.Deployment); err == nil {
			for _, c := range input.Deployment.Status.Conditions {
				if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
					return true
				}
			}
		}
		return false
	}, intervals...).Should(BeTrue(), func() string { return DescribeFailedDeployment(ctx, input) })
	Logf("Deployment %s/%s is now available, took %v", namespace, name, time.Since(start))
}

// GetWaitForDeploymentsAvailableInput is a convenience func to compose a WaitForDeploymentsAvailableInput
func GetWaitForDeploymentsAvailableInput(ctx context.Context, clusterProxy framework.ClusterProxy, name, namespace string, specName string) WaitForDeploymentsAvailableInput {
	Expect(clusterProxy).NotTo(BeNil())
	cl := clusterProxy.GetClient()
	var d = &appsv1.Deployment{}
	Eventually(func() error {
		return cl.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, d)
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())
	clientset := clusterProxy.GetClientSet()
	return WaitForDeploymentsAvailableInput{
		Deployment: d,
		Clientset:  clientset,
		Getter:     cl,
	}
}

// DescribeFailedDeployment returns detailed output to help debug a deployment failure in e2e.
func DescribeFailedDeployment(ctx context.Context, input WaitForDeploymentsAvailableInput) string {
	namespace, name := input.Deployment.GetNamespace(), input.Deployment.GetName()
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("Deployment %s/%s failed",
		namespace, name))
	b.WriteString(fmt.Sprintf("\nDeployment:\n%s\n", prettyPrint(input.Deployment)))
	b.WriteString(describeEvents(ctx, input.Clientset, namespace, name))
	return b.String()
}

// describeEvents returns a string summarizing recent events involving the named object(s).
func describeEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	b := strings.Builder{}
	if clientset == nil {
		b.WriteString("clientset is nil, so skipping output of relevant events")
	} else {
		opts := metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
			Limit:         20,
		}
		evts, err := clientset.CoreV1().Events(namespace).List(ctx, opts)
		if err != nil {
			b.WriteString(err.Error())
		} else {
			w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', tabwriter.FilterHTML)
			fmt.Fprintln(w, "LAST SEEN\tTYPE\tREASON\tOBJECT\tMESSAGE")
			for _, e := range evts.Items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s/%s\t%s\n", e.LastTimestamp, e.Type, e.Reason,
					strings.ToLower(e.InvolvedObject.Kind), e.InvolvedObject.Name, e.Message)
			}
			w.Flush()
		}
	}
	return b.String()
}

// prettyPrint returns a formatted JSON version of the object given.
func prettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func getHelmActionConfigForTests(ctx context.Context, workloadClusterProxy framework.ClusterProxy) *helmAction.Configuration {

	workloadKubeconfigPath := workloadClusterProxy.GetKubeconfigPath()

	settings := helmCli.New()
	settings.KubeConfig = workloadKubeconfigPath

	actionConfig := new(helmAction.Configuration)
	klog.Info("Initializing action config")
	err := actionConfig.Init(settings.RESTClientGetter(), "default", "secret", Logf)
	Expect(err).NotTo(HaveOccurred())

	return actionConfig
}

// GetWaitForHelmReleaseDeployedInput is a convenience func to compose a WaitForHelmReleaseDeployedInput.
func GetWaitForHelmReleaseDeployedInput(ctx context.Context, workloadClusterProxy framework.ClusterProxy, releaseName, namespace string, specName string) WaitForHelmReleaseDeployedInput {
	Expect(workloadClusterProxy).NotTo(BeNil())

	// Workaround atm so we don't need to deal with generated random Helm release names
	Expect(releaseName).NotTo(BeEmpty())

	actionConfig := getHelmActionConfigForTests(ctx, workloadClusterProxy)

	var release *helmRelease.Release
	Eventually(func() error {
		getClient := helmAction.NewGet(actionConfig)
		r, err := getClient.Run(releaseName)
		if err == helmDriver.ErrReleaseNotFound {
			return errors.Wrapf(err, "Helm release `%s` not found", releaseName)
		} else if err != nil {
			return err
		}
		if r == nil {
			return errors.Errorf("Helm release `%s` is nil, this is unexpected", releaseName)
		}

		release = r

		return nil
	}, e2eConfig.GetIntervals(specName, "wait-helm-release")...).Should(Succeed())

	return WaitForHelmReleaseDeployedInput{
		ActionConfig: actionConfig,
		Namespace:    namespace,
		HelmRelease:  release,
	}
}

// WaitForHelmReleaseDeployedInput is the input for WaitForHelmReleaseDeployed.
type WaitForHelmReleaseDeployedInput struct {
	ActionConfig *helmAction.Configuration
	HelmRelease  *helmRelease.Release
	Namespace    string
}

// WaitForHelmReleaseDeployed waits until the Helm release has status.Status = deployed, which signals that the Helm release was successfully deployed.
func WaitForHelmReleaseDeployed(ctx context.Context, input WaitForHelmReleaseDeployedInput, intervals ...interface{}) {
	start := time.Now()
	Expect(input.HelmRelease).ToNot(BeNil())
	getClient := helmAction.NewGet(input.ActionConfig)

	Log("starting to wait for Helm release to be deployed")
	Eventually(func() bool {
		if release, err := getClient.Run(input.HelmRelease.Name); err == nil {
			if release != nil && release.Info.Status == helmRelease.StatusDeployed {
				return true
			}
		} else {
			input.HelmRelease = release
		}

		return false
	}, intervals...).Should(BeTrue(), fmt.Sprintf("HelmRelease %s/%s failed to deploy, status: %s", input.Namespace, input.HelmRelease.Name, input.HelmRelease.Info.Status))
	Logf("Helm release %s is now deployed, took %v", input.HelmRelease, time.Since(start))
}

// WaitForHelmReleaseProxyReadyInput is the input for WaitForHelmReleaseProxyReady.
type WaitForHelmReleaseProxyReadyInput struct {
	Getter           framework.Getter
	HelmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
	Client           ctrlclient.Client
}

// WaitForHelmReleaseProxyReady waits until the HelmReleaseProxy has ready condition = True, that signals that the Helm
// install was successful.
func WaitForHelmReleaseProxyReady(ctx context.Context, input WaitForHelmReleaseProxyReadyInput, intervals ...interface{}) {
	start := time.Now()
	namespace, name := input.HelmReleaseProxy.GetNamespace(), input.HelmReleaseProxy.GetName()

	Byf("waiting for HelmReleaseProxy for %s/%s to be ready", input.HelmReleaseProxy.GetNamespace(), input.HelmReleaseProxy.GetName())
	Log("starting to wait for HelmReleaseProxy to become available")
	Eventually(func() bool {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		if err := input.Getter.Get(ctx, key, input.HelmReleaseProxy); err == nil {
			if conditions.IsTrue(input.HelmReleaseProxy, clusterv1.ReadyCondition) {
				return true
			}
		}
		return false
	}, intervals...).Should(BeTrue(), fmt.Sprintf("HelmReleaseProxy %s/%s failed to with ready condition: %+v`", namespace, name, conditions.Get(input.HelmReleaseProxy, clusterv1.ReadyCondition)))
	Logf("HelmReleaseProxy %s/%s is now ready, took %v", namespace, name, time.Since(start))
}

// GetWaitForHelmReleaseProxyReadyInput is a convenience func to compose a WaitForHelmReleaseProxyReadyInput.
func GetWaitForHelmReleaseProxyReadyInput(ctx context.Context, clusterProxy framework.ClusterProxy, clusterName string, helmChartProxy addonsv1alpha1.HelmChartProxy, specName string) WaitForHelmReleaseProxyReadyInput {
	Expect(clusterProxy).NotTo(BeNil())
	cl := clusterProxy.GetClient()
	var helmReleaseProxy *addonsv1alpha1.HelmReleaseProxy
	Eventually(func() error {
		if hrp, err := getHelmReleaseProxy(ctx, cl, clusterName, helmChartProxy); err != nil {
			return err
		} else {
			helmReleaseProxy = hrp
		}

		return nil
	}, e2eConfig.GetIntervals(specName, "wait-helmreleaseproxy")...).Should(Succeed())
	return WaitForHelmReleaseProxyReadyInput{
		HelmReleaseProxy: helmReleaseProxy,
		Getter:           cl,
	}
}

// logCheckpoint prints a message indicating the start or end of the current test spec,
// including which Ginkgo node it's running on.
//
// Example output:
//
//	INFO: "With 1 worker node" started at Tue, 22 Sep 2020 13:19:08 PDT on Ginkgo node 2 of 3
//	INFO: "With 1 worker node" ran for 18m34s on Ginkgo node 2 of 3
func logCheckpoint(specTimes map[string]time.Time) {
	text := CurrentSpecReport().LeafNodeText
	start, started := specTimes[text]
	suiteConfig, reporterConfig := GinkgoConfiguration()
	if !started {
		start = time.Now()
		specTimes[text] = start
		fmt.Fprintf(GinkgoWriter, "INFO: \"%s\" started at %s on Ginkgo node %d of %d and junit test report to file %s\n", text,
			start.Format(time.RFC1123), GinkgoParallelProcess(), suiteConfig.ParallelTotal, reporterConfig.JUnitReport)
	} else {
		elapsed := time.Since(start)
		fmt.Fprintf(GinkgoWriter, "INFO: \"%s\" ran for %s on Ginkgo node %d of %d and reported junit test to file %s\n", text,
			elapsed.Round(time.Second), GinkgoParallelProcess(), suiteConfig.ParallelTotal, reporterConfig.JUnitReport)
	}
}

func getHelmReleaseProxy(ctx context.Context, c ctrlclient.Client, clusterName string, helmChartProxy addonsv1alpha1.HelmChartProxy) (*addonsv1alpha1.HelmReleaseProxy, error) {
	// Get the HelmRelease
	releaseList := &addonsv1alpha1.HelmReleaseProxyList{}
	labels := map[string]string{
		clusterv1.ClusterNameLabel:             clusterName,
		addonsv1alpha1.HelmChartProxyLabelName: helmChartProxy.Name,
	}
	if err := c.List(ctx, releaseList, client.InNamespace(helmChartProxy.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	if len(releaseList.Items) != 1 {
		return nil, errors.Errorf("expected 1 HelmReleaseProxy, got %d", len(releaseList.Items))
	}

	return &releaseList.Items[0], nil
}
