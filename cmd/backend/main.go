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

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"
	"github.com/sethvargo/go-envconfig"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Options for the CLI.
type Options struct {
	Log    LogConfig `env:", prefix=EPHEMERAL_LOG_"`
	Server ServerConfig
}

type ServerConfig struct {
	Port       int    `env:"EPHEMERAL_SERVER_PORT, default=8888"`
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

func newClient(kubeconfig string, logger log.Logger) (*dynamic.DynamicClient, error) {
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

	dynCli, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating k8s dynamic client: %w", err)
	}

	return dynCli, nil
}

func readEnvConfigs() (*Options, error) {
	var config Options
	err := envconfig.Process(context.Background(), &config)
	if err != nil {
		return nil, fmt.Errorf("error reading configs from environment: %w", err)
	}
	return &config, nil
}

func main() {
	opts, err := readEnvConfigs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error retrieving configurations: %s\n", err)
		os.Exit(1)
	}

	level := log.LogLevel(opts.Log.Level)
	format := log.LogFormat(opts.Log.Format)
	logger, err := log.New(log.WithLevel(level), log.WithFormat(format))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating logger: %s\n", err)
		os.Exit(1)
	}

	dynClient, err := newClient(opts.Server.Kubeconfig, logger)
	if err != nil {
		logger.Error(err, "newClient error")
		os.Exit(1)
	}

	c := backend.NewK8sClient(dynClient)
	service := backend.NewDefaultService(c, logger)
	handler := backend.NewAPIHandler(service, logger)

	cli := humacli.New(func(hooks humacli.Hooks, options *ServerConfig) {
		router := chi.NewMux()
		api := humachi.New(router, huma.DefaultConfig(backend.APITitle, backend.APIVersion))
		backend.RegisterRoutes(api, handler)

		server := http.Server{
			Addr:    fmt.Sprintf(":%d", opts.Server.Port),
			Handler: router,
		}

		hooks.OnStart(func() {
			logger.Info("Starting Ephemeral Access API Server...", "port", opts.Server.Port)
			server.ListenAndServe()
		})
		// graceful shutdown the server
		hooks.OnStop(func() {
			// Give the server 10 seconds to gracefully shut down, then give up.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			server.Shutdown(ctx)
		})
	})
	cli.Run()
}
