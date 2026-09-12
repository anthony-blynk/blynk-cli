// Package cmd implements the blynk-cli command tree.
package cmd

import (
	"fmt"
	"os"

	"github.com/anthony-blynk/blynk-cli/internal/api"
	"github.com/anthony-blynk/blynk-cli/internal/config"
	"github.com/spf13/cobra"
)

// Global flag values, bound in root's PersistentFlags.
var (
	flagServer       string
	flagToken        string
	flagClientID     string
	flagClientSecret string
	flagProfile      string
	flagOrgID        int64
	flagOutput       string
	flagQuiet        bool
	flagPage         int
	flagSize         int
	flagAll          bool
	flagYes          bool
)

// RootCmd is the entry point for the blynk-cli command tree.
var RootCmd = &cobra.Command{
	Use:           "blynk",
	Short:         "Command-line client for the Blynk Platform API",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	pf := RootCmd.PersistentFlags()
	pf.StringVar(&flagServer, "server", "", "Blynk server domain (only needed if not using a profile)")
	pf.StringVar(&flagToken, "token", os.Getenv("BLYNK_TOKEN"), "Bearer access token (or $BLYNK_TOKEN)")
	pf.StringVar(&flagClientID, "client-id", os.Getenv("BLYNK_CLIENT_ID"), "OAuth2 client id (or $BLYNK_CLIENT_ID)")
	pf.StringVar(&flagClientSecret, "client-secret", os.Getenv("BLYNK_CLIENT_SECRET"), "OAuth2 client secret (or $BLYNK_CLIENT_SECRET)")
	pf.StringVar(&flagProfile, "profile", "", "Named profile to use for this call only")
	pf.Int64Var(&flagOrgID, "org-id", 0, "Override org ID for this call")
	pf.StringVarP(&flagOutput, "output", "o", "table", "Output format: table|json|yaml")
	pf.BoolVarP(&flagQuiet, "quiet", "q", false, "Minimal output (e.g. just \"online\"/\"offline\")")
	pf.IntVar(&flagPage, "page", 0, "0-indexed page")
	pf.IntVar(&flagSize, "size", 50, "Page size, max 1000")
	pf.BoolVar(&flagAll, "all", false, "Auto-paginate, fetch every page")
	pf.BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation prompts")
}

// Execute runs the root command.
func Execute() error {
	return RootCmd.Execute()
}

// resolveOptsFromFlags builds a config.ResolveOptions from the global flags.
func resolveOptsFromFlags() config.ResolveOptions {
	return config.ResolveOptions{
		ProfileFlag:  flagProfile,
		Server:       flagServer,
		Token:        flagToken,
		ClientID:     flagClientID,
		ClientSecret: flagClientSecret,
	}
}

// requireClient resolves the active profile per the documented precedence
// order, ensures a valid bearer token, and returns a ready-to-use API
// client along with the config (nil profileName if flags bypassed profiles).
func requireClient() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	resolved, err := config.Resolve(cfg, resolveOptsFromFlags())
	if err != nil {
		return nil, err
	}

	token, err := config.EnsureToken(cfg, resolved.ProfileName, &resolved.Profile)
	if err != nil {
		return nil, err
	}

	return api.NewClient(resolved.Profile.Server, token), nil
}

// loadConfig loads config.yaml, exiting with a clear error on failure.
func loadConfig() (*config.Config, error) {
	return config.Load()
}

// printErr writes a user-facing error to stderr.
func printErr(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
}
