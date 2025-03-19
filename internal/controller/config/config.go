package config

import (
	"context"
	"fmt"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
)

// Configurer defines the accessor methods for all configurations that can
// be provided externally to the ephemeral access controller process. The
// main purpose behind this interface is to ensure that externally provided
// configuration can not be changed once retrieved.
type Configurer interface {
	LogConfigurer
	MetricsConfigurer
	ControllerConfigurer
	PluginConfigurer
}

// LogConfigurer defines the accessor methods for log configurations.
type LogConfigurer interface {
	LogLevel() string
	LogFormat() string
}

// PluginConfigurer defines the accessor methods for the plugin configurations.
type PluginConfigurer interface {
	PluginPath() string
}

// MetricsConfigurer defines the accessor methods for metrics configurations.
type MetricsConfigurer interface {
	MetricsAddress() string
	MetricsSecure() bool
}

// ControllerConfigurer defines the accessor methods for the controller's
// configurations.
type ControllerConfigurer interface {
	EnableLeaderElection() bool
	ControllerPort() int
	ControllerHealthProbeAddr() string
	ControllerEnableHTTP2() bool
	ControllerRequeueInterval() time.Duration
}

// MetricsAddress acessor method
func (c *Config) MetricsAddress() string {
	return c.Metrics.Address
}

// MetricsSecure acessor method
func (c *Config) MetricsSecure() bool {
	return c.Metrics.Secure
}

// LogLevel acessor method
func (c *Config) LogLevel() string {
	return c.Log.Level
}

// LogFormat acessor method
func (c *Config) LogFormat() string {
	return c.Log.Format
}

// EnableLeaderElection acessor method
func (c *Config) EnableLeaderElection() bool {
	return c.Controller.EnableLeaderElection
}

// ControllerPort acessor method
func (c *Config) ControllerPort() int {
	return c.Controller.Port
}

// ControllerHealthProbeAddr acessor method
func (c *Config) ControllerHealthProbeAddr() string {
	return c.Controller.HealthProbeAddr
}

// ControllerEnableHTTP2 acessor method
func (c *Config) ControllerEnableHTTP2() bool {
	return c.Controller.EnableHTTP2
}

// ControllerRequeueInterval acessor method
func (c *Config) ControllerRequeueInterval() time.Duration {
	return c.Controller.RequeueInterval
}

// PluginPath acessor method
func (c *Config) PluginPath() string {
	return c.Plugin.Path
}

// Config defines all configurations available for this controller
type Config struct {
	// Metrics defines the metrics configurations
	Metrics MetricsConfig `env:", prefix=EPHEMERAL_METRICS_"`
	// Log defines the logs configurations
	Log LogConfig `env:", prefix=EPHEMERAL_LOG_"`
	// Controller defines the controller configurations
	Controller ControllerConfig `env:", prefix=EPHEMERAL_CONTROLLER_"`
	Plugin     PluginConfig     `env:", prefix=EPHEMERAL_PLUGIN_"`
}

// PluginConfig defines the plugin configuration
type PluginConfig struct {
	// Path must be the full path to the binary implementing the plugin interface
	Path string `env:"PATH"`
}

// MetricsConfig defines the metrics configurations
type MetricsConfig struct {
	// Address The address the metric endpoint binds to.
	// Can be set to 0 in order to disable the metrics server
	Address string `env:"ADDR, default=:8090"`
	// Secure If set the metrics endpoint is served securely.
	Secure bool `env:"SECURE, default=false"`
}

// ControllerConfig defines the controller configurations
type ControllerConfig struct {
	// Port The controller main port for routes such as pprof
	Port int `env:"PORT, default=8081"`
	// EnableLeaderElection Enable leader election for controller manager.
	// Enabling this will ensure there is only one active controller manager.
	EnableLeaderElection bool `env:"ENABLE_LEADER_ELECTION, default=false"`
	// HealthProbeAddr The address the probe endpoint binds to.
	HealthProbeAddr string `env:"HEALTH_PROBE_ADDR, default=:8082"`
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
	// Possible values: text, json
	// Default: text
	Format string `env:"FORMAT, default=text"`
}

// String prints the config state
func (c *Config) String() string {
	return fmt.Sprintf(
		"Metrics: [ Address: %s Secure: %t ] Log [ Level: %s Format: %s ] Controller [ EnableLeaderElection: %t HealthProbeAddress: %s EnableHTTP2: %t RequeueInterval: %s ] Plugin [ Path : %s ]",
		c.Metrics.Address,
		c.Metrics.Secure,
		c.Log.Level,
		c.Log.Format,
		c.Controller.EnableLeaderElection,
		c.Controller.HealthProbeAddr,
		c.Controller.EnableHTTP2,
		c.Controller.RequeueInterval,
		c.Plugin.Path,
	)
}

// ReadEnvConfigs will read all environment variables as defined in the Config
// struct and return a Configurer interface which provides accessor methods for
// all configurations.
func ReadEnvConfigs() (Configurer, error) {
	var config Config
	err := envconfig.Process(context.Background(), &config)
	if err != nil {
		return nil, fmt.Errorf("envconfig.Process error: %w", err)
	}
	return &config, nil
}
