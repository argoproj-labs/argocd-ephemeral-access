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

package backend

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/argoproj-labs/argocd-ephemeral-access/internal/backend"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/backend/metrics"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"
	"github.com/sethvargo/go-envconfig"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Options for the CLI.
type Options struct {
	Log     LogConfig `env:", prefix=EPHEMERAL_LOG_"`
	Backend BackendConfig
}

// BackendConfig defines the exposed backend configurations
type BackendConfig struct {
	// Port defines the port used to listen to http requests sent to this service
	Port int `env:"EPHEMERAL_BACKEND_PORT, default=8888"`
	// MetricPort defined the port used to expose http request metrics
	MetricsPort int `env:"EPHEMERAL_BACKEND_METRICS_PORT, default=8883"`
	// Kubeconfig is an optional configuration to allow connecting to a k8s cluster
	// remotelly
	Kubeconfig string `env:"KUBECONFIG"`
	// Namespace must point to the namespace where this backend service is running
	Namespace string `env:"EPHEMERAL_BACKEND_NAMESPACE, required"`
	// DefaultAccessDuration defines the default duration to be used when creating
	// AccessRequests
	DefaultAccessDuration time.Duration `env:"EPHEMERAL_BACKEND_DEFAULT_ACCESS_DURATION, default=4h"`
}

// LogConfig defines the log configurations
type LogConfig struct {
	// Level defines the log level.
	// Possible values: debug, info, error
	// Default: info
	Level string `env:"LEVEL, default=info"`
	// Format defines the log output format.
	// Possible values: text, json
	// Default: text
	Format string `env:"FORMAT, default=text"`
}

func newRestConfig(kubeconfig string, logger log.Logger) (*rest.Config, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		logger.Info("Kubernetes client: Using in-cluster configuration")
		config, err = rest.InClusterConfig()
	} else {
		logger.Info(fmt.Sprintf("Kubernetes client: Using configurations from %q", kubeconfig))
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		return nil, fmt.Errorf("error building k8s rest config: %w", err)
	}
	return config, nil
}

func readEnvConfigs() (*Options, error) {
	var config Options
	err := envconfig.Process(context.Background(), &config)
	if err != nil {
		return nil, fmt.Errorf("error reading configs from environment: %w", err)
	}
	return &config, nil
}

func NewCommand() *cobra.Command {
	command := cobra.Command{
		Use:               "backend",
		Short:             "Run the Ephemeral Access backend service",
		Long:              "The Argo CD Ephemeral Access extension requires this backend service to operate. It serves the REST API used by the UI extension.",
		DisableAutoGenTag: true,
		RunE:              run,
	}
	return &command
}

func run(cmd *cobra.Command, args []string) error {
	opts, err := readEnvConfigs()
	if err != nil {
		return fmt.Errorf("error retrieving configurations: %s", err)
	}

	level := log.LogLevel(opts.Log.Level)
	format := log.LogFormat(opts.Log.Format)
	logger, err := log.New(log.WithLevel(level), log.WithFormat(format))
	if err != nil {
		return fmt.Errorf("error creating logger: %s", err)
	}

	restConfig, err := newRestConfig(opts.Backend.Kubeconfig, logger)
	if err != nil {
		return fmt.Errorf("error creating new rest config: %w", err)
	}
	persister, err := backend.NewK8sPersister(restConfig, logger)
	if err != nil {
		return fmt.Errorf("error creating a new k8s persister: %w", err)
	}

	service := backend.NewDefaultService(persister, logger, opts.Backend.Namespace, opts.Backend.DefaultAccessDuration)
	handler := backend.NewAPIHandler(service, logger)

	cli := humacli.New(func(hooks humacli.Hooks, options *BackendConfig) {
		router := chi.NewMux()
		router.Use(backend.MetricsMiddleware)
		api := humachi.New(router, huma.DefaultConfig(backend.APITitle, backend.APIVersion))
		backend.RegisterRoutes(api, handler)

		server := http.Server{
			Addr:    fmt.Sprintf(":%d", opts.Backend.Port),
			Handler: router,
		}

		// Metrics server
		metricsRouter := chi.NewMux()
		metricsRouter.Handle("/metrics", metrics.MetricsHandler())
		metricsServer := http.Server{
			Addr:    fmt.Sprintf(":%d", opts.Backend.MetricsPort),
			Handler: metricsRouter,
		}

		ctx, cancel := context.WithCancel(context.Background())
		hooks.OnStart(func() {
			defer cancel()
			cacheErr := make(chan error)

			go func() {
				defer close(cacheErr)
				err := persister.StartCache(ctx)
				if err != nil {
					cacheErr <- fmt.Errorf("start persister cache error: %w", err)
				}
			}()
			serverErr := make(chan error)

			go func() {
				defer close(serverErr)
				logger.Info("Starting Ephemeral Access API Server...", "configs", opts)
				err := server.ListenAndServe()
				if err != nil {
					serverErr <- fmt.Errorf("server error: %w", err)
				}
			}()
			metricsErr := make(chan error)

			go func() {
				defer close(metricsErr)
				logger.Info("Starting Metrics Server...", "port", opts.Backend.MetricsPort)
				if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Error(err, "metrics server error")
				}
			}()
			select {
			case <-ctx.Done():
				logger.Info("Stopping Ephemeral Access API Server: Context done")
				logger.Info("Stopping Metrics Server: Context done")
				return
			case err := <-cacheErr:
				logger.Error(err, "cache error")
				shutdownServer(&server, logger)
				shutdownServer(&metricsServer, logger)
			case err := <-serverErr:
				logger.Error(err, "server error")
				shutdownServer(&metricsServer, logger)
			case err := <-metricsErr:
				logger.Error(err, "metrics server error")
				shutdownServer(&server, logger)
			}
		})
		// graceful shutdown the server
		hooks.OnStop(func() {
			cancel()
			logger.Info("Shutting down Ephemeral Access API Server: Context done")
			shutdownServer(&server, logger)
			logger.Info("Shutting down Metrics Server: Context done")
			shutdownServer(&metricsServer, logger)
		})
	})
	cli.Run()
	return nil
}

func shutdownServer(server *http.Server, logger log.Logger) {
	// Give the server 10 seconds to gracefully shut down, then give up.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error(err, "Error during server shutdown", "address", server.Addr)
	} else {
		logger.Info("Server shutdown completed successfully", "address", server.Addr)
	}
}
