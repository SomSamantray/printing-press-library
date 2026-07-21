// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config holds CLI configuration including cookie auth.
type Config struct {
	BaseURL    string            `toml:"base_url"`
	AuthHeader string            `toml:"auth_header"`
	Headers    map[string]string `toml:"headers,omitempty"`
	Path       string            `toml:"-"`
}

// DefaultPath returns ~/.config/notebooklm-pp-cli/config.toml
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "notebooklm-pp-cli", "config.toml"), nil
}

// Load reads config from disk.
func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	cfg := &Config{
		BaseURL: nlmBaseURL,
		Path:    path,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = nlmBaseURL
	}
	return cfg, nil
}

// Save persists config.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.Path), 0o700); err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.Path, data, 0o600)
}

// HTTPClient returns an http.Client that injects the Cookie header on every request.
func (c *Config) HTTPClient() (*http.Client, error) {
	client := &http.Client{}
	if c.AuthHeader == "" {
		return client, nil
	}
	client.Transport = &cookieTransport{cookie: strings.TrimSpace(c.AuthHeader)}
	return client, nil
}

const nlmBaseURL = "https://notebooklm.google.com"

type cookieTransport struct {
	cookie string
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header.Set("Cookie", t.cookie)
	return http.DefaultTransport.RoundTrip(cloned)
}

// ValidateAuthHeader reports whether auth_header looks populated.
func (c *Config) ValidateAuthHeader() error {
	if strings.TrimSpace(c.AuthHeader) == "" {
		return fmt.Errorf("auth_header is empty")
	}
	if !strings.Contains(c.AuthHeader, "SID=") {
		return fmt.Errorf("auth_header missing SID cookie")
	}
	return nil
}
