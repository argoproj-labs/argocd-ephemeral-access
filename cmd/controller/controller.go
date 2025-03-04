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
	"crypto/tls"
	"fmt"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	argocd "github.com/argoproj-labs/argocd-ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/controller"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/controller/config"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
	"github.com/spf13/cobra"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(api.AddToScheme(scheme))
	utilruntime.Must(argocd.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func NewCommand() *cobra.Command {
	command := cobra.Command{
		Use:               "controller",
		Short:             "Run the Ephemeral Access Controller",
		Long:              "The Argo CD Ephemeral Access extension requires this controller to operate. It reconciles AccessRequest CRDs.",
		DisableAutoGenTag: true,
		RunE:              run,
	}
	return &command
}

func run(cmd *cobra.Command, args []string) error {
	config, err := config.ReadEnvConfigs()
	if err != nil {
		return fmt.Errorf("error retrieving configurations: %s", err)
	}

	level := log.LogLevel(config.LogLevel())
	format := log.LogFormat(config.LogFormat())
	logger, err := log.NewLogger(log.WithLevel(level), log.WithFormat(format))
	if err != nil {
		return fmt.Errorf("error creating logger: %s", err)
	}

	ctrl.SetLogger(logger)

	setupLog.Info(fmt.Sprintf("Using controller configs: %s", config))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !config.ControllerEnableHTTP2() {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   config.MetricsAddress(),
			SecureServing: config.MetricsSecure(),
			TLSOpts:       tlsOpts,
		},
		PprofBindAddress:       fmt.Sprintf(":%d", config.ControllerPort()),
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: config.ControllerHealthProbeAddr(),
		LeaderElection:         config.EnableLeaderElection(),
		LeaderElectionID:       "8246dd0c.argoproj-labs.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	service := controller.NewService(mgr.GetClient(), config)

	if err = (&controller.AccessRequestReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Service: service,
		Config:  config,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller AccessRequest controller: %w", err)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}
	return nil
}
