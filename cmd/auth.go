package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/anthony-blynk/blynk-cli/internal/api"
	"github.com/anthony-blynk/blynk-cli/internal/config"
	"github.com/anthony-blynk/blynk-cli/internal/output"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate and inspect the current session",
}

func init() {
	RootCmd.AddCommand(authCmd)
	authCmd.AddCommand(
		authLoginCmd(),
		authTokenCmd(),
		authWhoamiCmd(),
		authLogoutCmd(),
	)
}

func authLoginCmd() *cobra.Command {
	var server, clientID, clientSecret, profileName string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate and save a profile (scriptable equivalent of `profile add --use`)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if server == "" {
				return fmt.Errorf("--server is required")
			}
			if clientID == "" {
				return fmt.Errorf("--client-id is required")
			}
			if clientSecret == "" {
				secret, err := promptSecret("Client secret")
				if err != nil {
					return err
				}
				clientSecret = secret
			}

			p := config.Profile{Server: server, ClientID: clientID, ClientSecret: clientSecret}
			tok, expiry, err := api.FetchToken(p.Server, p.ClientID, p.ClientSecret)
			if err != nil {
				return fmt.Errorf("validate credentials: %w", err)
			}
			p.CachedToken = tok
			p.TokenExpiry = expiry

			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			cfg.Set(profileName, p)
			cfg.CurrentProfile = profileName
			if err := cfg.Save(); err != nil {
				return err
			}

			fmt.Printf("Logged in, saved as profile %q and set as current.\n", profileName)
			return nil
		},
	}

	cmd.Flags().StringVar(&server, "server", "", "Blynk server domain")
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth2 client id")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "OAuth2 client secret (prompted if omitted)")
	cmd.Flags().StringVar(&profileName, "profile", "default", "Profile name to save credentials under")
	return cmd
}

func authTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print the current cached bearer token",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			resolved, err := config.Resolve(cfg, resolveOptsFromFlags())
			if err != nil {
				return err
			}
			token, err := config.EnsureToken(cfg, resolved.ProfileName, &resolved.Profile)
			if err != nil {
				return err
			}
			fmt.Println(token)
			return nil
		},
	}
}

func authWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Resolve the current token to org info",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := requireClient()
			if err != nil {
				return err
			}
			org, err := client.OrganizationProfile()
			if err != nil {
				return err
			}

			t := &output.Table{Headers: []string{"FIELD", "VALUE"}}
			t.Rows = append(t.Rows,
				[]string{"org_id", fmt.Sprintf("%d", org.ID)},
				[]string{"org_name", org.Name},
				[]string{"server", client.Server},
			)
			return output.Render(os.Stdout, flagOutput, org, t)
		},
	}
}

func authLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear the current profile and its cached token",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if cfg.CurrentProfile == "" {
				fmt.Println("Not logged in.")
				return nil
			}

			name := cfg.CurrentProfile
			if p, ok := cfg.Get(name); ok {
				p.CachedToken = ""
				p.TokenExpiry = time.Time{}
				cfg.Set(name, p)
			}
			cfg.CurrentProfile = ""
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("Logged out of %q.\n", name)
			return nil
		},
	}
}
