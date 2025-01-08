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

package controllers_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	helmRelease "helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"sigs.k8s.io/cluster-api-addon-provider-helm/controllers/helmchartproxy"
	"sigs.k8s.io/cluster-api-addon-provider-helm/controllers/helmreleaseproxy"
	"sigs.k8s.io/cluster-api-addon-provider-helm/internal/mocks"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient  client.Client
	testEnv    *envtest.Environment
	k8sManager manager.Manager
	ctx        context.Context
	cancel     context.CancelFunc
	helmClient *mocks.MockClient
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ gomock.TestReporter = (*TestReporter)(nil)

type TestReporter struct{}

func (c TestReporter) Errorf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

func (c TestReporter) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

	fs := flag.FlagSet{}
	klog.InitFlags(&fs)
	err := fs.Set("v", "2")
	Expect(err).NotTo(HaveOccurred())
	ctrl.SetLogger(textlogger.NewLogger(textlogger.NewConfig()))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "tmp", "cluster-api", "crd"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = addonsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = clusterv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	for _, namespace := range namespaces {
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).NotTo(HaveOccurred())
	}

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	helmClient = mocks.NewMockClient(gomock.NewController(&TestReporter{}))

	helmClient.EXPECT().InstallOrUpgradeHelmRelease(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(helmReleaseDeployed, nil).AnyTimes()
	helmClient.EXPECT().GetHelmRelease(gomock.Any(), gomock.Any(), gomock.Any()).Return(&helmRelease.Release{}, nil).AnyTimes()
	helmClient.EXPECT().UninstallHelmRelease(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_, _, _ any) (*helmRelease.UninstallReleaseResponse, error) {
		if failedHelmUninstall {
			return nil, errors.New(releaseFailedMessage)
		}

		return &helmRelease.UninstallReleaseResponse{}, nil
	},
	).AnyTimes()

	err = (&helmchartproxy.HelmChartProxyReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(ctx, k8sManager, controller.Options{})
	Expect(err).ToNot(HaveOccurred())

	err = (&helmreleaseproxy.HelmReleaseProxyReconciler{
		Client:     k8sManager.GetClient(),
		Scheme:     k8sManager.GetScheme(),
		HelmClient: helmClient,
	}).SetupWithManager(ctx, k8sManager, controller.Options{})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
