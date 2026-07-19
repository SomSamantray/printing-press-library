package config

type Config struct {
	Version string `toml:"version"`
}

func Load() (*Config, error) {
	return &Config{Version: "1.0.0"}, nil
}
