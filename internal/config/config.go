package config

import (
	"github.com/caarlos0/env/v11"

	"mesa-ads/internal/config/configs"
)

// Config aggregates all configuration sections for the application. Fields
// are populated from environment variables using the caarlos0/env library. The
// nested structs are tagged with envPrefix so their fields are parsed with
// the given prefix. See the individual types in the configs package for
// default values and options. Use Load to construct a Config.
type Config struct {
	// Env specifies the deployment environment (e.g. prod, dev). It is not
	// currently used by the application but may be useful for logging or
	// metrics.
	Env string `env:"ENV" envDefault:"prod"`

	// HTTP holds configuration for the HTTP server. Environment variables
	// prefixed with HTTP_ will populate this struct.
	HTTP configs.HTTP `envPrefix:"HTTP_"`

	// Log configures the structured logger. Environment variables prefixed
	// with LOG_ will populate this struct.
	Log configs.Logger `envPrefix:"LOG_"`

	// Psql configures the PostgreSQL connection. Environment variables
	// prefixed with PSQL_ will populate this struct.
	Psql configs.Postgres `envPrefix:"PSQL_"`
}

// Load reads configuration from environment variables into a Config. If
// parsing fails, an error is returned. All fields are loaded with their
// specified defaults when no environment variable is provided.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
