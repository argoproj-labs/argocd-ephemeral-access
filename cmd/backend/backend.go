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

	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
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

type BackendConfig struct {
	Port       int    `env:"EPHEMERAL_BACKEND_PORT, default=8888"`
	Kubeconfig string `env:"KUBECONFIG"`
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

	service := backend.NewDefaultService(persister, logger, "TODO: get namespace from config or env var")
	handler := backend.NewAPIHandler(service, logger)

	cli := humacli.New(func(hooks humacli.Hooks, options *BackendConfig) {
		router := chi.NewMux()
		api := humachi.New(router, huma.DefaultConfig(backend.APITitle, backend.APIVersion))
		backend.RegisterRoutes(api, handler)

		server := http.Server{
			Addr:    fmt.Sprintf(":%d", opts.Backend.Port),
			Handler: router,
		}

		ctx, cancel := context.WithCancel(context.Background())
		hooks.OnStart(func() {
			defer cancel()
			cacheErr := make(chan error)
			defer close(cacheErr)
			go func() {
				err := persister.StartCache(ctx)
				if err != nil {
					cacheErr <- fmt.Errorf("start persister cache error: %w", err)
				}
			}()
			serverErr := make(chan error)
			defer close(serverErr)
			go func() {
				logger.Info("Starting Ephemeral Access API Server...", "port", opts.Backend.Port)
				server.ListenAndServe()
			}()
			select {
			case <-ctx.Done():
				logger.Info("Stopping Ephemeral Access API Server: Context done")
				return
			case err := <-cacheErr:
				shutdownServer(&server)
				logger.Error(err, "cache error")
			case err := <-serverErr:
				logger.Error(err, "server error")
			}
		})
		// graceful shutdown the server
		hooks.OnStop(func() {
			cancel()
			shutdownServer(&server)
		})
	})
	cli.Run()
	return nil
}

func shutdownServer(server *http.Server) {
	// Give the server 10 seconds to gracefully shut down, then give up.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}
