package config

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	FolderID    string       `mapstructure:"folder_id"`
	AuthKeyFile string       `mapstructure:"auth_key_file"`
	DefaultTTL  int64        `mapstructure:"default_ttl"`
	LogLevel    string       `mapstructure:"log_level"`
	LogFormat   string       `mapstructure:"log_format"`
	Server      ServerConfig `mapstructure:"server"`
}

type ServerConfig struct {
	WebhookPort int `mapstructure:"webhook_port"`
	HealthPort  int `mapstructure:"health_port"`
}

func LoadConfig() (*Config, error) {
	// Configure Viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Environment variable settings
	viper.AutomaticEnv()

	// Define CLI flags
	pflag.String("folder-id", "", "Yandex Cloud folder ID")
	pflag.String("auth-key-file", "/etc/kubernetes/key.json", "Path to Yandex Cloud service account key file")
	pflag.Int("webhook-port", 8888, "Port for webhook server")
	pflag.Int("health-port", 8080, "Port for health check server")
	pflag.Int64("default-ttl", 300, "Default record TTL (seconds) used when an endpoint has no TTL set")
	pflag.String("log-level", "info", "Log level (trace, debug, info, warn, error, fatal, panic)")
	pflag.String("log-format", "json", "Log format (json or text)")
	pflag.Parse()

	// Bind CLI flags to Viper
	if err := viper.BindPFlag("folder_id", pflag.Lookup("folder-id")); err != nil {
		return nil, fmt.Errorf("error binding folder-id flag: %v", err)
	}
	if err := viper.BindPFlag("auth_key_file", pflag.Lookup("auth-key-file")); err != nil {
		return nil, fmt.Errorf("error binding auth-key-file flag: %v", err)
	}
	if err := viper.BindPFlag("server.webhook_port", pflag.Lookup("webhook-port")); err != nil {
		return nil, fmt.Errorf("error binding webhook-port flag: %v", err)
	}
	if err := viper.BindPFlag("server.health_port", pflag.Lookup("health-port")); err != nil {
		return nil, fmt.Errorf("error binding health-port flag: %v", err)
	}
	if err := viper.BindPFlag("default_ttl", pflag.Lookup("default-ttl")); err != nil {
		return nil, fmt.Errorf("error binding default-ttl flag: %v", err)
	}
	if err := viper.BindPFlag("log_level", pflag.Lookup("log-level")); err != nil {
		return nil, fmt.Errorf("error binding log-level flag: %v", err)
	}
	if err := viper.BindPFlag("log_format", pflag.Lookup("log-format")); err != nil {
		return nil, fmt.Errorf("error binding log-format flag: %v", err)
	}

	// Set default values
	viper.SetDefault("server.webhook_port", 8888)
	viper.SetDefault("server.health_port", 8080)
	viper.SetDefault("default_ttl", 300)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("log_format", "json")

	// Read configuration file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %v", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %v", err)
	}

	if err := validate(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validate checks required fields and the log configuration. It is kept
// separate from LoadConfig so it can be unit-tested without touching the
// global viper/pflag state that LoadConfig mutates.
func validate(c *Config) error {
	if c.FolderID == "" {
		return fmt.Errorf("folder_id configuration is required")
	}

	if c.AuthKeyFile == "" {
		return fmt.Errorf("auth_key_file configuration is required")
	}

	if _, err := log.ParseLevel(c.LogLevel); err != nil {
		return fmt.Errorf("invalid log_level %q: %v", c.LogLevel, err)
	}

	if c.LogFormat != "json" && c.LogFormat != "text" {
		return fmt.Errorf("invalid log_format %q: must be \"json\" or \"text\"", c.LogFormat)
	}

	return nil
}
