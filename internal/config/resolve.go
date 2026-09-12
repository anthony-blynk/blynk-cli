package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const profileFileName = ".blynk-profile"

// ResolveOptions carries the global flag values that can influence which
// profile/credentials a command uses.
type ResolveOptions struct {
	ProfileFlag  string // --profile
	Server       string // --server
	Token        string // --token
	ClientID     string // --client-id
	ClientSecret string // --client-secret
}

// Resolved is the effective connection info for a command invocation.
type Resolved struct {
	ProfileName string // empty when flags bypassed profiles entirely
	Profile     Profile
}

// Resolve implements the precedence order documented in CLAUDE.md:
//  1. --profile flag
//  2. fully-specified --server/--token or --server/--client-id/--client-secret flags
//  3. $BLYNK_PROFILE
//  4. .blynk-profile file (cwd or a parent dir)
//  5. current_profile in config.yaml
func Resolve(cfg *Config, opts ResolveOptions) (*Resolved, error) {
	// 1. --profile flag (one-off, doesn't change current_profile)
	if opts.ProfileFlag != "" {
		return resolveNamed(cfg, opts.ProfileFlag)
	}

	// 2. fully-specified flags bypass profiles entirely.
	if opts.Server != "" && (opts.Token != "" || (opts.ClientID != "" && opts.ClientSecret != "")) {
		return &Resolved{Profile: Profile{
			Server:       opts.Server,
			Token:        opts.Token,
			ClientID:     opts.ClientID,
			ClientSecret: opts.ClientSecret,
		}}, nil
	}

	// 3. $BLYNK_PROFILE
	if name := os.Getenv("BLYNK_PROFILE"); name != "" {
		return resolveNamed(cfg, name)
	}

	// 4. .blynk-profile file in cwd or a parent dir.
	if name, ok := findProfileFile(); ok {
		return resolveNamed(cfg, name)
	}

	// 5. current_profile from config.yaml
	if cfg.CurrentProfile != "" {
		return resolveNamed(cfg, cfg.CurrentProfile)
	}

	return nil, fmt.Errorf("no profile selected: use --profile, $BLYNK_PROFILE, a .blynk-profile file, `blynk profile use`, or fully-specified --server/--token flags")
}

func resolveNamed(cfg *Config, name string) (*Resolved, error) {
	resolvedName, err := cfg.Resolve(name)
	if err != nil {
		return nil, err
	}
	p, ok := cfg.Get(resolvedName)
	if !ok {
		return nil, fmt.Errorf("profile %q not found", resolvedName)
	}
	return &Resolved{ProfileName: resolvedName, Profile: p}, nil
}

// findProfileFile walks up from the current working directory looking for
// a .blynk-profile file, returning its (trimmed) contents.
func findProfileFile() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		candidate := filepath.Join(dir, profileFileName)
		if data, err := os.ReadFile(candidate); err == nil {
			data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // strip UTF-8 BOM
			name := strings.TrimSpace(string(data))
			if name != "" {
				return name, true
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
