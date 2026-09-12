package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/anthony-blynk/blynk-cli/internal/api"
	"github.com/anthony-blynk/blynk-cli/internal/output"
	"github.com/spf13/cobra"
)

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Inspect devices",
}

func init() {
	RootCmd.AddCommand(deviceCmd)
	deviceCmd.AddCommand(
		deviceListCmd(),
		deviceGetCmd(),
	)
}

func deviceListCmd() *cobra.Command {
	var includeSubOrgDevices bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List devices",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := requireClient()
			if err != nil {
				return err
			}

			var devices []api.Device
			page := flagPage
			for {
				batch, total, err := client.ListDevices(flagOrgID, includeSubOrgDevices, page, flagSize)
				if err != nil {
					return err
				}
				devices = append(devices, batch...)
				if !flagAll || len(batch) == 0 || len(devices) >= int(total) {
					break
				}
				page++
			}

			t := &output.Table{Headers: []string{"ID", "NAME", "TEMPLATE", "FIRMWARE", "BOARD"}}
			for _, d := range devices {
				version, board := "", ""
				if d.HardwareInfo != nil {
					version = d.HardwareInfo.Version
					board = d.HardwareInfo.BoardType
				}
				t.Rows = append(t.Rows, []string{
					strconv.FormatInt(d.ID, 10),
					d.Name,
					strconv.FormatInt(int64(d.TemplateID), 10),
					version,
					board,
				})
			}
			return output.Render(os.Stdout, flagOutput, devices, t)
		},
	}

	cmd.Flags().BoolVar(&includeSubOrgDevices, "include-sub-org-devices", false, "Include devices from sub-organizations")
	return cmd
}

func deviceGetCmd() *cobra.Command {
	var idFlag string
	var reveal bool

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show a device's details, including live online status",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if idFlag == "" {
				return fmt.Errorf("--id is required")
			}

			client, err := requireClient()
			if err != nil {
				return err
			}

			resolved, err := resolveDeviceToken(client, idFlag)
			if err != nil {
				return fmt.Errorf("resolve device %q: %w", idFlag, err)
			}

			// Always re-fetch via the single-device endpoint so output is
			// consistent regardless of whether --id was a numeric id
			// (already the full schema) or a name (resolved via search,
			// which omits lifecycleStatus/connect metadata).
			d, err := client.GetDevice(resolved.ID)
			if err != nil {
				return err
			}

			online, err := client.IsOnline(d.ID)
			if err != nil {
				return err
			}

			if flagQuiet {
				if online {
					fmt.Println("online")
				} else {
					fmt.Println("offline")
				}
				return nil
			}

			display := *d
			if !reveal {
				display.Token = "" // device auth credential — never print by default
			}

			return output.Render(os.Stdout, flagOutput, &display, deviceTable(&display, online, reveal))
		},
	}

	cmd.Flags().StringVar(&idFlag, "id", "", "Device ID or name (required)")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Show the device's auth token")
	return cmd
}

func deviceTable(d *api.Device, online bool, reveal bool) *output.Table {
	onlineStr := "offline"
	if online {
		onlineStr = "online"
	}

	t := &output.Table{Headers: []string{"FIELD", "VALUE"}}
	t.Rows = append(t.Rows,
		[]string{"id", strconv.FormatInt(d.ID, 10)},
		[]string{"name", d.Name},
		[]string{"template_id", strconv.FormatInt(int64(d.TemplateID), 10)},
		[]string{"online", onlineStr},
	)
	if d.HardwareInfo != nil {
		h := d.HardwareInfo
		t.Rows = append(t.Rows,
			[]string{"firmware_version", h.Version},
			[]string{"firmware_build", h.Build},
			[]string{"board_type", h.BoardType},
			[]string{"blynk_version", h.BlynkVersion},
		)
	}
	if d.LifecycleStatus != nil {
		t.Rows = append(t.Rows, []string{"lifecycle_status", d.LifecycleStatus.Name})
	}
	if reveal && d.Token != "" {
		t.Rows = append(t.Rows, []string{"token", d.Token})
	}
	return t
}
