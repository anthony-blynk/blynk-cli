package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/anthony-blynk/blynk-cli/internal/api"
	"github.com/anthony-blynk/blynk-cli/internal/output"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Inspect and invite users",
}

func init() {
	RootCmd.AddCommand(userCmd)
	userCmd.AddCommand(
		userListCmd(),
		userGetCmd(),
		userInviteCmd(),
	)
}

func userListCmd() *cobra.Command {
	var includeSubOrgUsers bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := requireClient()
			if err != nil {
				return err
			}

			var users []api.User
			page := flagPage
			for {
				batch, total, err := client.ListUsers(includeSubOrgUsers, page, flagSize)
				if err != nil {
					return err
				}
				users = append(users, batch...)
				if !flagAll || len(batch) == 0 || len(users) >= int(total) {
					break
				}
				page++
			}

			t := &output.Table{Headers: []string{"ID", "NAME", "EMAIL", "ROLE_ID", "DEV"}}
			for _, u := range users {
				dev := ""
				if u.IsDev {
					dev = "yes"
				}
				t.Rows = append(t.Rows, []string{
					strconv.FormatInt(u.ID, 10),
					u.Name,
					u.Email,
					strconv.FormatInt(int64(u.RoleID), 10),
					dev,
				})
			}
			return output.Render(os.Stdout, flagOutput, users, t)
		},
	}

	cmd.Flags().BoolVar(&includeSubOrgUsers, "include-sub-org-users", false, "Include users from sub-organizations")
	return cmd
}

func userGetCmd() *cobra.Command {
	var idFlag string

	var cmd *cobra.Command
	cmd = &cobra.Command{
		Use:   "get [id]",
		Short: "Show a user's details",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			idFlag, err := resolveIdentifier(args, cmd.Flags().Changed("id"), idFlag, "id")
			if err != nil {
				return err
			}

			client, err := requireClient()
			if err != nil {
				return err
			}

			resolved, err := resolveUserToken(client, idFlag)
			if err != nil {
				return fmt.Errorf("resolve user %q: %w", idFlag, err)
			}

			// Always re-fetch via the single-user endpoint so output is
			// consistent regardless of whether idFlag was a numeric id
			// (already the full schema) or a name/email (resolved via
			// search, which returns the lighter list/search schema).
			d, err := client.GetUser(resolved.ID)
			if err != nil {
				return err
			}

			return output.Render(os.Stdout, flagOutput, d, userTable(d))
		},
	}

	cmd.Flags().StringVar(&idFlag, "id", "", "User ID, name, or email")
	return cmd
}

func userInviteCmd() *cobra.Command {
	var email, name, locale string
	var roleID int32

	cmd := &cobra.Command{
		Use:   "invite",
		Short: "Invite a new user to the organization by email",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if roleID == 0 {
				return fmt.Errorf("--role-id is required")
			}

			if !confirm(fmt.Sprintf("Invite %s <%s> to the organization with role id %d?", name, email, roleID)) {
				fmt.Println("Cancelled.")
				return nil
			}

			client, err := requireClient()
			if err != nil {
				return err
			}

			u, err := client.InviteUser(api.InviteUserRequest{
				Email:  email,
				Name:   name,
				RoleID: roleID,
				OrgID:  flagOrgID,
				Locale: locale,
			})
			if err != nil {
				return err
			}

			fmt.Printf("✓ Invited %s <%s> (user id %d, status %s)\n", u.Name, u.Email, u.ID, u.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email address to invite (required)")
	cmd.Flags().StringVar(&name, "name", "", "Name for the invited user (required)")
	cmd.Flags().Int32Var(&roleID, "role-id", 0, "Role ID to assign (required — no lookup-by-name is available, see CLAUDE.md)")
	cmd.Flags().StringVar(&locale, "locale", "", "Locale for the invite (optional)")
	return cmd
}

// resolveUserToken resolves a --id value to a User: a numeric token is
// looked up directly, anything else is treated as a name or email and
// resolved via the search endpoint (which must yield exactly one match,
// preferring an exact case-insensitive name/email match over a substring
// one) — mirroring resolveDeviceToken/resolveTemplateToken.
func resolveUserToken(client *api.Client, token string) (*api.User, error) {
	if id, err := strconv.ParseInt(token, 10, 64); err == nil {
		d, err := client.GetUser(id)
		if err != nil {
			return nil, err
		}
		return &d.User, nil
	}

	results, err := client.SearchUsers(token)
	if err != nil {
		return nil, fmt.Errorf("search for user %q: %w", token, err)
	}

	var exact []api.User
	for _, u := range results {
		if strings.EqualFold(u.Name, token) || strings.EqualFold(u.Email, token) {
			exact = append(exact, u)
		}
	}
	switch {
	case len(exact) == 1:
		return &exact[0], nil
	case len(exact) > 1:
		return nil, fmt.Errorf("ambiguous user %q, candidates: %s", token, userCandidates(exact))
	case len(results) == 1:
		return &results[0], nil
	case len(results) > 1:
		return nil, fmt.Errorf("ambiguous user %q, candidates: %s", token, userCandidates(results))
	default:
		return nil, fmt.Errorf("no user found matching %q", token)
	}
}

func userCandidates(users []api.User) string {
	labels := make([]string, len(users))
	for i, u := range users {
		labels[i] = fmt.Sprintf("%s <%s> (id %d)", u.Name, u.Email, u.ID)
	}
	return strings.Join(labels, ", ")
}

func userTable(d *api.UserDetails) *output.Table {
	t := &output.Table{Headers: []string{"FIELD", "VALUE"}}
	t.Rows = append(t.Rows,
		[]string{"id", strconv.FormatInt(d.ID, 10)},
		[]string{"name", d.Name},
		[]string{"email", d.Email},
		// No documented endpoint resolves role_id to a role name for an
		// arbitrary user with org/client-scoped auth; see internal/api/users.go.
		[]string{"role_id", strconv.FormatInt(int64(d.RoleID), 10)},
		[]string{"org_id", strconv.FormatInt(d.OrgID, 10)},
		[]string{"status", d.Status},
	)
	if d.Title != "" {
		t.Rows = append(t.Rows, []string{"title", d.Title})
	}
	if d.NickName != "" {
		t.Rows = append(t.Rows, []string{"nick_name", d.NickName})
	}
	if d.PhoneNumber != "" {
		t.Rows = append(t.Rows, []string{"phone_number", d.PhoneNumber})
	}
	if d.TZ != "" {
		t.Rows = append(t.Rows, []string{"tz", d.TZ})
	}
	if d.Locale != "" {
		t.Rows = append(t.Rows, []string{"locale", d.Locale})
	}
	t.Rows = append(t.Rows, []string{"is_dev", strconv.FormatBool(d.IsDev)})
	if d.LastLoggedAt != 0 {
		t.Rows = append(t.Rows, []string{"last_logged_at", formatMillis(d.LastLoggedAt)})
	}
	if d.RegisteredAt != 0 {
		t.Rows = append(t.Rows, []string{"registered_at", formatMillis(d.RegisteredAt)})
	}
	return t
}

func formatMillis(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02 15:04:05 UTC")
}
