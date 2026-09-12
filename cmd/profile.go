package cmd

import (
	"fmt"
	"os"

	"github.com/anthony-blynk/blynk-cli/internal/api"
	"github.com/anthony-blynk/blynk-cli/internal/config"
	"github.com/anthony-blynk/blynk-cli/internal/output"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage saved server/credential profiles",
}

func init() {
	RootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(
		profileAddCmd(),
		profileListCmd(),
		profileShowCmd(),
		profileUseCmd(),
		profileRemoveCmd(),
		profileRenameCmd(),
		profileSwitchCmd(),
	)
}

func profileAddCmd() *cobra.Command {
	var server, clientID, clientSecret, token string
	var useIt bool

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			if server == "" {
				return fmt.Errorf("--server is required")
			}
			if clientID != "" && token != "" {
				return fmt.Errorf("--client-id and --token are mutually exclusive")
			}
			if clientID == "" && token == "" {
				return fmt.Errorf("specify either --client-id (OAuth2) or --token (static token)")
			}

			p := config.Profile{Server: server}

			if clientID != "" {
				p.ClientID = clientID
				p.ClientSecret = clientSecret
				if p.ClientSecret == "" {
					secret, err := promptSecret("Client secret")
					if err != nil {
						return err
					}
					p.ClientSecret = secret
				}

				tok, expiry, err := api.FetchToken(p.Server, p.ClientID, p.ClientSecret)
				if err != nil {
					return fmt.Errorf("validate credentials: %w", err)
				}
				p.CachedToken = tok
				p.TokenExpiry = expiry
			} else {
				p.Token = token
				client := api.NewClient(p.Server, p.Token)
				if _, err := client.OrganizationProfile(); err != nil {
					return fmt.Errorf("validate token: %w", err)
				}
			}

			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			cfg.Set(name, p)
			if useIt {
				cfg.CurrentProfile = name
			}
			if err := cfg.Save(); err != nil {
				return err
			}

			fmt.Printf("Profile %q saved.\n", name)
			if useIt {
				fmt.Printf("Now using %q.\n", name)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&server, "server", "", "Blynk server domain")
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth2 client id")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "OAuth2 client secret (prompted if omitted)")
	cmd.Flags().StringVar(&token, "token", "", "Static access token")
	cmd.Flags().BoolVar(&useIt, "use", false, "Set as the current profile")
	return cmd
}

func profileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved profiles",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			names := cfg.Names()
			t := &output.Table{Headers: []string{"NAME", "SERVER", "AUTH", "CURRENT"}}
			for _, n := range names {
				p := cfg.Profiles[n]
				current := ""
				if n == cfg.CurrentProfile {
					current = "*"
				}
				t.Rows = append(t.Rows, []string{n, p.Server, authType(p), current})
			}
			return output.Render(os.Stdout, flagOutput, names, t)
		},
	}
}

func profileShowCmd() *cobra.Command {
	var reveal bool

	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show a profile's details",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			name, err := cfg.Resolve(args[0])
			if err != nil {
				return err
			}
			p := cfg.Profiles[name]

			t := &output.Table{Headers: []string{"FIELD", "VALUE"}}
			t.Rows = append(t.Rows, []string{"name", name})
			t.Rows = append(t.Rows, []string{"server", p.Server})
			t.Rows = append(t.Rows, []string{"auth", authType(p)})
			if p.ClientID != "" {
				t.Rows = append(t.Rows, []string{"client_id", p.ClientID})
			}

			secret := "(hidden, use --reveal)"
			if reveal {
				if p.Token != "" {
					secret = p.Token
				} else {
					secret = p.ClientSecret
				}
			}
			if p.Token != "" {
				t.Rows = append(t.Rows, []string{"token", secret})
			} else if p.ClientSecret != "" {
				t.Rows = append(t.Rows, []string{"client_secret", secret})
			}

			return output.Render(os.Stdout, flagOutput, p, t)
		},
	}
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Show the secret/token value")
	return cmd
}

func profileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the current profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			name, err := cfg.Resolve(args[0])
			if err != nil {
				return err
			}
			cfg.CurrentProfile = name
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("Now using %q.\n", name)
			return nil
		},
	}
}

func profileRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			name, err := cfg.Resolve(args[0])
			if err != nil {
				return err
			}
			if !confirm(fmt.Sprintf("Remove profile %q?", name)) {
				fmt.Println("Cancelled.")
				return nil
			}
			cfg.Remove(name)
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("Profile %q removed.\n", name)
			return nil
		},
	}
}

func profileRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			oldName, err := cfg.Resolve(args[0])
			if err != nil {
				return err
			}
			newName := args[1]
			if _, exists := cfg.Get(newName); exists {
				return fmt.Errorf("profile %q already exists", newName)
			}

			wasCurrent := cfg.CurrentProfile == oldName

			p := cfg.Profiles[oldName]
			cfg.Set(newName, p)
			cfg.Remove(oldName)
			if wasCurrent {
				cfg.CurrentProfile = newName
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("Profile %q renamed to %q.\n", oldName, newName)
			return nil
		},
	}
}

func profileSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch",
		Short: "Interactively pick the current profile",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			names := cfg.Names()
			if len(names) == 0 {
				return fmt.Errorf("no profiles saved yet, use `blynk profile add`")
			}
			name, err := pickProfile(names)
			if err != nil {
				return err
			}
			cfg.CurrentProfile = name
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("Now using %q.\n", name)
			return nil
		},
	}
}

func authType(p config.Profile) string {
	if p.IsStaticToken() {
		return "static-token"
	}
	return "client-credentials"
}
