// Package cmd implements the blynk-cli command tree.
package cmd

import (
	"fmt"
	"os"
	"strconv"

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

// Version, Commit, and Date are set via -ldflags at build time (see
// .goreleaser.yaml); "dev"/"none"/"unknown" are the `go build`/`go run`
// defaults for a non-release build.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// RootCmd is the entry point for the blynk-cli command tree.
var RootCmd = &cobra.Command{
	Use:           "blynk",
	Short:         "Command-line client for the Blynk Platform API",
	Version:       fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date),
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

// resolveIdentifier reconciles a bare positional argument with a flag that
// identifies the same single resource — every blynk-cli command that
// identifies exactly one resource (unlike e.g. a future `tag assign
// --tag-id --device-id`, which genuinely needs two and would be ambiguous
// as positional args) accepts either its flag or a positional argument,
// never both (unless they agree) and never neither.
func resolveIdentifier(args []string, flagChanged bool, flagValue, flagName string) (string, error) {
	switch {
	case len(args) == 1 && flagChanged:
		if args[0] != flagValue {
			return "", fmt.Errorf("both a positional argument (%q) and --%s (%q) were given; use only one", args[0], flagName, flagValue)
		}
		return flagValue, nil
	case len(args) == 1:
		return args[0], nil
	case flagChanged:
		return flagValue, nil
	default:
		return "", fmt.Errorf("--%s or a positional argument is required", flagName)
	}
}

// resolveIdentifierInt64 is resolveIdentifier for int64-flagged commands
// (e.g. shipment ids), which have no name-resolution path so the value must
// parse cleanly as an integer either way.
func resolveIdentifierInt64(args []string, flagChanged bool, flagValue int64, flagName string) (int64, error) {
	switch {
	case len(args) == 1 && flagChanged:
		parsed, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid %s %q", flagName, args[0])
		}
		if parsed != flagValue {
			return 0, fmt.Errorf("both a positional argument (%d) and --%s (%d) were given; use only one", parsed, flagName, flagValue)
		}
		return flagValue, nil
	case len(args) == 1:
		parsed, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid %s %q", flagName, args[0])
		}
		return parsed, nil
	case flagChanged:
		return flagValue, nil
	default:
		return 0, fmt.Errorf("--%s or a positional argument is required", flagName)
	}
}
