/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	// "github.com/onsi/gomega/gexec"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	argocd "github.com/argoproj-labs/argocd-ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/plugin"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/mocks"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	dynClient            *dynamic.DynamicClient
	restConfig           *rest.Config
	k8sClient            client.Client
	testEnv              *envtest.Environment
	cancel               context.CancelFunc
	controllerConfigMock *mocks.MockControllerConfigurer
	accessRequesterMock  *mocks.MockAccessRequester
)

func TestControllers(t *testing.T) {
	controllerConfigMock = mocks.NewMockControllerConfigurer(t)
	accessRequesterMock = mocks.NewMockAccessRequester(t)
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	accessRequestCRDPath := filepath.Join("..", "..", "config", "crd", "bases")
	argoprojCRDPath := filepath.Join("..", "..", "test", "manifests", "argoproj", "crd", "schema")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{accessRequestCRDPath, argoprojCRDPath},
		ErrorIfCRDPathMissing: true,

		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.30.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	var err error
	// cfg is defined in this file globally.
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	// +kubebuilder:scaffold:scheme
	err = api.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = argocd.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	ctx, c := context.WithCancel(context.Background())
	cancel = c

	k8sClient, err = client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: ":9393",
		},
	})
	Expect(err).ToNot(HaveOccurred())

	controllerConfigMock.EXPECT().ControllerPort().Return(8081).Maybe()
	controllerConfigMock.EXPECT().ControllerHealthProbeAddr().Return(":8082").Maybe()
	controllerConfigMock.EXPECT().ControllerEnableHTTP2().Return(false).Maybe()
	controllerConfigMock.EXPECT().ControllerRequeueInterval().Return(time.Second * 1).Maybe()
	controllerConfigMock.EXPECT().ControllerRequestTimeout().Return(time.Second * 5).Maybe()
	controllerConfigMock.EXPECT().ControllerAccessRequestTTL().Return(time.Second * 3).Maybe()

	pluginResponse := &plugin.GrantResponse{
		Status: plugin.GrantStatusGranted,
	}
	accessRequesterMock.EXPECT().GrantAccess(mock.Anything, mock.Anything).Return(pluginResponse, nil).Maybe()
	accessRequesterMock.EXPECT().RevokeAccess(mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	service := NewService(k8sManager.GetClient(), controllerConfigMock, accessRequesterMock)
	arReconciler := &AccessRequestReconciler{
		Client:  k8sManager.GetClient(),
		Scheme:  k8sManager.GetScheme(),
		Service: service,
		Config:  controllerConfigMock,
	}
	err = arReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	dynClient, err = dynamic.NewForConfig(restConfig)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to start k8s manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
