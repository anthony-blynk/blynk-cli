// Package config handles reading and writing ~/.blynk/config.yaml, the
// profile store used by every blynk-cli command.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Profile is a single named connection: either OAuth2 client-credentials
// (Server + ClientID + ClientSecret) or a static token (Server + Token).
type Profile struct {
	Server       string `yaml:"server"`
	ClientID     string `yaml:"client_id,omitempty"`
	ClientSecret string `yaml:"client_secret,omitempty"`
	Token        string `yaml:"token,omitempty"`

	// Cached OAuth2 access token for client-credentials profiles.
	CachedToken string    `yaml:"cached_token,omitempty"`
	TokenExpiry time.Time `yaml:"token_expiry,omitempty"`
}

// IsStaticToken reports whether this profile authenticates with a fixed
// token rather than OAuth2 client-credentials.
func (p Profile) IsStaticToken() bool {
	return p.Token != "" && p.ClientID == ""
}

// Config is the on-disk shape of ~/.blynk/config.yaml.
type Config struct {
	CurrentProfile string             `yaml:"current_profile,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles"`

	// path this config was loaded from/will be saved to; not serialized.
	path string `yaml:"-"`
}

// Dir returns the ~/.blynk directory path for the current OS user.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".blynk"), nil
}

// Path returns the full path to config.yaml.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads config.yaml, returning an empty Config if it doesn't exist yet.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	cfg := &Config{Profiles: map[string]Profile{}, path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // strip UTF-8 BOM

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.path = path
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return cfg, nil
}

// Save writes the config back to disk, creating ~/.blynk if needed and
// restricting its permissions.
func (c *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := c.path
	if path == "" {
		if path, err = Path(); err != nil {
			return err
		}
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Get returns the named profile.
func (c *Config) Get(name string) (Profile, bool) {
	p, ok := c.Profiles[name]
	return p, ok
}

// Set stores or replaces a profile.
func (c *Config) Set(name string, p Profile) {
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	c.Profiles[name] = p
}

// Remove deletes a profile, clearing CurrentProfile if it pointed at it.
func (c *Config) Remove(name string) {
	delete(c.Profiles, name)
	if c.CurrentProfile == name {
		c.CurrentProfile = ""
	}
}

// Names returns all profile names, sorted.
func (c *Config) Names() []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Resolve matches name against profile names, accepting an unambiguous
// prefix or substring match. It returns the exact profile name on success.
func (c *Config) Resolve(name string) (string, error) {
	if _, ok := c.Profiles[name]; ok {
		return name, nil
	}

	all := c.Names()
	var prefixMatches, substrMatches []string
	lower := strings.ToLower(name)
	for _, n := range all {
		ln := strings.ToLower(n)
		if strings.HasPrefix(ln, lower) {
			prefixMatches = append(prefixMatches, n)
		} else if strings.Contains(ln, lower) {
			substrMatches = append(substrMatches, n)
		}
	}

	switch {
	case len(prefixMatches) == 1:
		return prefixMatches[0], nil
	case len(prefixMatches) > 1:
		return "", fmt.Errorf("ambiguous profile %q, candidates: %s", name, strings.Join(prefixMatches, ", "))
	case len(substrMatches) == 1:
		return substrMatches[0], nil
	case len(substrMatches) > 1:
		return "", fmt.Errorf("ambiguous profile %q, candidates: %s", name, strings.Join(substrMatches, ", "))
	default:
		return "", fmt.Errorf("no profile matches %q", name)
	}
}
