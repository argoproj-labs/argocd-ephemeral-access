package config

import (
	"context"
	"fmt"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
)

type Configurer interface {
	LogConfigurer
	MetricsConfigurer
	ControllerConfigurer
}
type LogConfigurer interface {
	LogLevel() string
	LogFormat() string
}
type MetricsConfigurer interface {
	MetricsAddress() string
	MetricsSecure() bool
}
type ControllerConfigurer interface {
	EnableLeaderElection() bool
	ControllerHealthProbeAddr() string
	ControllerEnableHTTP2() bool
	ControllerRequeueInterval() time.Duration
}

func (c *Config) MetricsAddress() string {
	return c.Metrics.Address
}
func (c *Config) MetricsSecure() bool {
	return c.Metrics.Secure
}
func (c *Config) LogLevel() string {
	return c.Log.Level
}
func (c *Config) LogFormat() string {
	return c.Log.Format
}
func (c *Config) EnableLeaderElection() bool {
	return c.Controller.EnableLeaderElection
}
func (c *Config) ControllerHealthProbeAddr() string {
	return c.Controller.HealthProbeAddr
}
func (c *Config) ControllerEnableHTTP2() bool {
	return c.Controller.EnableHTTP2
}
func (c *Config) ControllerRequeueInterval() time.Duration {
	return c.Controller.RequeueInterval
}

// Config defines all configurations available for this controller
type Config struct {
	// Metrics defines the metrics configurations
	Metrics MetricsConfig `env:", prefix=EPHEMERAL_METRICS_"`
	// Log defines the logs configurations
	Log LogConfig `env:", prefix=EPHEMERAL_LOG_"`
	// Controller defines the controller configurations
	Controller ControllerConfig `env:", prefix=EPHEMERAL_CONTROLLER_"`
}

// MetricsConfig defines the metrics configurations
type MetricsConfig struct {
	// Address The address the metric endpoint binds to.
	// Use the port :8080. If not set, it will be 0 in order to disable the metrics server
	Address string `env:"ADDR, default=0"`
	// Secure If set the metrics endpoint is served securely.
	Secure bool `env:"SECURE, default=false"`
}

// ControllerConfig defines the controller configurations
type ControllerConfig struct {
	// EnableLeaderElection Enable leader election for controller manager.
	// Enabling this will ensure there is only one active controller manager.
	EnableLeaderElection bool `env:"ENABLE_LEADER_ELECTION, default=false"`
	// HealthProbeAddr The address the probe endpoint binds to.
	HealthProbeAddr string `env:"HEALTH_PROBE_ADDR, default=:8081"`
	// EnableHTTP2 If set, HTTP/2 will be enabled for the metrics and webhook
	// servers.
	EnableHTTP2 bool `env:"ENABLE_HTTP2, default=false"`
	// RequeueInterval determines the interval the controller will requeue an
	// AccessRequest.
	// Valid time units are "ms", "s", "m", "h".
	// Default: 3 minutes
	RequeueInterval time.Duration `env:"REQUEUE_INTERVAL, default=3m"`
}

// LogConfig defines the log configurations
type LogConfig struct {
	// Level defines the log level.
	// Possible values: debug, info, error
	// Default: info
	Level string `env:"LEVEL, default=info"`
	// Format defines the log output format.
	// Possible values: console, json
	// Default: console
	Format string `env:"FORMAT, default=console"`
}

func (c *Config) String() string {
	return fmt.Sprintf("Metrics: [ Address: %s Secure: %t ] Log [ Level: %s Format: %s ] Controller [ EnableLeaderElection: %t HealthProbeAddress: %s EnableHTTP2: %t RequeueInterval: %s]", c.Metrics.Address, c.Metrics.Secure, c.Log.Level, c.Log.Format, c.Controller.EnableLeaderElection, c.Controller.HealthProbeAddr, c.Controller.EnableHTTP2, c.Controller.RequeueInterval)
}

func NewConfiguration() (Configurer, error) {
	var config Config
	err := envconfig.Process(context.Background(), &config)
	if err != nil {
		return nil, fmt.Errorf("envconfig.Process error: %w", err)
	}
	return &config, nil
}
