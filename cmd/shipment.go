package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/anthony-blynk/blynk-cli/internal/api"
	"github.com/anthony-blynk/blynk-cli/internal/output"
	"github.com/spf13/cobra"
)

var shipmentCmd = &cobra.Command{
	Use:   "shipment",
	Short: "Manage OTA firmware shipments",
}

func init() {
	RootCmd.AddCommand(shipmentCmd)
	shipmentCmd.AddCommand(
		shipmentListCmd(),
		shipmentGetCmd(),
		shipmentStopCmd(),
		shipmentDeleteCmd(),
		shipmentDeployCmd(),
	)
}

func shipmentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List shipments",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := requireClient()
			if err != nil {
				return err
			}
			shipments, err := client.ListShipments(flagOrgID)
			if err != nil {
				return err
			}

			t := &output.Table{Headers: []string{"ID", "TITLE", "STATUS", "TEMPLATE", "DEVICES", "SUCCESS"}}
			for _, s := range shipments {
				t.Rows = append(t.Rows, []string{
					strconv.FormatInt(s.ID, 10),
					s.Title,
					s.Status,
					fmt.Sprintf("%s (%d)", s.ProductName, s.ProductID),
					strconv.Itoa(len(s.DeviceIDs)),
					fmt.Sprintf("%d/%d", s.ShipmentProgress.Success, len(s.DeviceIDs)),
				})
			}
			return output.Render(os.Stdout, flagOutput, shipments, t)
		},
	}
}

func shipmentGetCmd() *cobra.Command {
	var id int64
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show a shipment's details",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := requireClient()
			if err != nil {
				return err
			}
			s, err := client.GetShipment(id, flagOrgID)
			if err != nil {
				return err
			}
			return output.Render(os.Stdout, flagOutput, s, shipmentTable(s))
		},
	}
	cmd.Flags().Int64Var(&id, "id", 0, "Shipment ID (required)")
	cmd.MarkFlagRequired("id")
	return cmd
}

func shipmentStopCmd() *cobra.Command {
	var id int64
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a running or paused shipment",
		Long: "Stop a running or paused shipment. The Platform API exposes a single " +
			"stop action (no separate pause/resume): only RUN or PAUSE shipments " +
			"can be stopped, and there is no way to resume one afterwards.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := requireClient()
			if err != nil {
				return err
			}
			s, err := client.StopShipment(id, flagOrgID)
			if err != nil {
				return err
			}
			fmt.Printf("Shipment %d stopped (status: %s).\n", s.ID, s.Status)
			return nil
		},
	}
	cmd.Flags().Int64Var(&id, "id", 0, "Shipment ID (required)")
	cmd.MarkFlagRequired("id")
	return cmd
}

func shipmentDeleteCmd() *cobra.Command {
	var id int64
	var force bool
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a finished or cancelled shipment",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !flagYes && !force {
				if !confirm(fmt.Sprintf("Delete shipment %d?", id)) {
					fmt.Println("Cancelled.")
					return nil
				}
			}
			client, err := requireClient()
			if err != nil {
				return err
			}
			if err := client.DeleteShipment(id, flagOrgID); err != nil {
				return err
			}
			fmt.Printf("Shipment %d deleted.\n", id)
			return nil
		},
	}
	cmd.Flags().Int64Var(&id, "id", 0, "Shipment ID (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.MarkFlagRequired("id")
	return cmd
}

func shipmentTable(s *api.Shipment) *output.Table {
	t := &output.Table{Headers: []string{"FIELD", "VALUE"}}
	p := s.ShipmentProgress
	t.Rows = append(t.Rows,
		[]string{"id", strconv.FormatInt(s.ID, 10)},
		[]string{"title", s.Title},
		[]string{"status", s.Status},
		[]string{"template", fmt.Sprintf("%s (%d)", s.ProductName, s.ProductID)},
		[]string{"firmware", s.FirmwareOriginalFileName},
		[]string{"devices", strconv.Itoa(len(s.DeviceIDs))},
		[]string{"started", strconv.Itoa(int(p.Started))},
		[]string{"request_sent", strconv.Itoa(int(p.RequestSent))},
		[]string{"firmware_requested", strconv.Itoa(int(p.FirmwareRequested))},
		[]string{"firmware_uploaded", strconv.Itoa(int(p.FirmwareUploaded))},
		[]string{"firmware_uploaded_to_mobile", strconv.Itoa(int(p.FirmwareUploadedToMobile))},
		[]string{"success", strconv.Itoa(int(p.Success))},
		[]string{"upload_failure", strconv.Itoa(int(p.UploadFailure))},
		[]string{"firmware_type_mismatch", strconv.Itoa(int(p.FirmwareTypeMismatch))},
		[]string{"download_limit_reached", strconv.Itoa(int(p.DownloadLimitReached))},
		[]string{"rollback", strconv.Itoa(int(p.Rollback))},
		[]string{"firmware_version_mismatch", strconv.Itoa(int(p.FirmwareVersionMismatch))},
	)
	return t
}

func shipmentDeployCmd() *cobra.Command {
	var file string
	var deviceIDsRaw string
	var templateIDFlag int32
	var name string
	var shipmentTime string
	var compareField string
	var skipFwTypeCheck bool
	var attemptsLimit int32
	var attemptResetPeriod time.Duration
	var wait, noWait bool
	var waitTimeout time.Duration
	var verbose bool
	var dryRun bool

	var cmd *cobra.Command
	cmd = &cobra.Command{
		Use:   "deploy",
		Short: "Upload firmware and create a shipment targeting it, in one step",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if file == "" {
				return fmt.Errorf("--file is required")
			}
			if deviceIDsRaw == "" {
				return fmt.Errorf("--device-ids is required")
			}

			client, err := requireClient()
			if err != nil {
				return err
			}

			devices, templateID, err := resolveDevices(client, deviceIDsRaw)
			if err != nil {
				return err
			}
			if templateIDFlag != 0 && templateIDFlag != templateID {
				return fmt.Errorf("--template-id %d does not match the template devices actually report (%d)", templateIDFlag, templateID)
			}

			for _, d := range devices {
				fmt.Printf("Resolved device %d → template %d (%s)\n", d.ID, d.TemplateID, deviceLabel(d))
			}

			if dryRun {
				fmt.Printf("Dry run: would deploy %s to %d device(s): %s\n", filepath.Base(file), len(devices), deviceListLabel(devices))
				return nil
			}

			if name == "" {
				name = autoShipmentName(devices, file)
			}

			target := deviceListLabel(devices)
			if !confirm(fmt.Sprintf("About to deploy %s to %d device(s) (%s) as shipment %q. Continue?", filepath.Base(file), len(devices), target, name)) {
				fmt.Println("Cancelled.")
				return nil
			}

			upload, err := client.UploadFirmware(file)
			if err != nil {
				return fmt.Errorf("upload firmware: %w", err)
			}
			info, err := os.Stat(file)
			if err == nil {
				fmt.Printf("✓ Uploaded %s (%s)\n", filepath.Base(file), humanBytes(info.Size()))
			}

			req := api.CreateShipmentRequest{
				OrgID:                    flagOrgID,
				ProductID:                templateID,
				Title:                    name,
				PathToFirmware:           upload.Path,
				FirmwareOriginalFileName: filepath.Base(file),
				FirmwareInfo:             upload.FirmwareInfo,
				DeviceIDs:                deviceIDs32(devices),
				ShipmentTime:             shipmentTime,
				SkipFwTypeCheck:          skipFwTypeCheck,
				CompareField:             compareField,
				AttemptsLimit:            attemptsLimit,
				AttemptResetPeriodMs:     attemptResetPeriod.Milliseconds(),
			}
			s, err := client.CreateShipment(req)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Shipment %d created\n", s.ID)

			effectiveWait := wait
			if !cmd.Flags().Changed("wait") && !cmd.Flags().Changed("no-wait") {
				effectiveWait = len(devices) == 1
			}
			if noWait {
				effectiveWait = false
			}
			if !effectiveWait {
				return nil
			}

			return waitForShipment(client, s.ID, len(devices), devices, waitTimeout, verbose)
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Firmware file to upload (required)")
	cmd.Flags().StringVar(&deviceIDsRaw, "device-ids", "", "Comma-separated device IDs or names to target (required)")
	cmd.Flags().Int32Var(&templateIDFlag, "template-id", 0, "Template ID to cross-check against the resolved devices")
	cmd.Flags().StringVar(&name, "name", "", "Shipment name (auto-generated if omitted)")
	cmd.Flags().StringVar(&shipmentTime, "shipment-time", "", "ANY|NIGHT|MORNING|AFTERNOON|EVENING (default ANY)")
	cmd.Flags().BoolVar(&skipFwTypeCheck, "skip-fw-type-check", false, "Bypass the device/template firmware-type compatibility check")
	cmd.Flags().StringVar(&compareField, "compare-field", "", "NO_CONDITION|BUILD_DATE_DIFFERS|EARLIER_BUILD_DATE|LATEST_FIRMWARE_VERSION|LATEST_BLYNK_VERSION (default BUILD_DATE_DIFFERS)")
	cmd.Flags().Int32Var(&attemptsLimit, "attempts-limit", 3, "Delivery attempts before giving up on a device (0 appears to mean the device is never notified)")
	cmd.Flags().DurationVar(&attemptResetPeriod, "attempt-reset-period", 24*time.Hour, "Window over which attempts-limit applies")
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until the rollout finishes (default on for a single device)")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Don't poll, return immediately after creating the shipment")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 15*time.Minute, "Max time to wait for rollout completion")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Print the full progress breakdown on every poll tick")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Resolve device targeting and print it, without uploading or creating anything")
	return cmd
}

// splitDeviceTokens splits --device-ids on commas; each token may be a
// numeric device id or a device name (resolved via a live search).
func splitDeviceTokens(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tokens = append(tokens, p)
		}
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("--device-ids must contain at least one device id or name")
	}
	return tokens, nil
}

func deviceIDs32(devices []api.Device) []int32 {
	out := make([]int32, len(devices))
	for i, d := range devices {
		out[i] = int32(d.ID)
	}
	return out
}

// resolveDeviceToken resolves one --device-ids token to a Device: a numeric
// token is looked up directly, anything else is treated as a device name
// and resolved via the search endpoint (which must yield exactly one match,
// preferring an exact case-insensitive name match over a substring one).
func resolveDeviceToken(client *api.Client, token string) (*api.Device, error) {
	if id, err := strconv.ParseInt(token, 10, 64); err == nil {
		return client.GetDevice(id)
	}

	results, err := client.SearchDevices(token)
	if err != nil {
		return nil, fmt.Errorf("search for device %q: %w", token, err)
	}

	var exact []api.Device
	for _, d := range results {
		if strings.EqualFold(d.Name, token) {
			exact = append(exact, d)
		}
	}
	switch {
	case len(exact) == 1:
		return &exact[0], nil
	case len(exact) > 1:
		return nil, fmt.Errorf("ambiguous device name %q, candidates: %s", token, deviceCandidates(exact))
	case len(results) == 1:
		return &results[0], nil
	case len(results) > 1:
		return nil, fmt.Errorf("ambiguous device name %q, candidates: %s", token, deviceCandidates(results))
	default:
		return nil, fmt.Errorf("no device found matching %q", token)
	}
}

func deviceCandidates(devices []api.Device) string {
	labels := make([]string, len(devices))
	for i, d := range devices {
		labels[i] = fmt.Sprintf("%s (id %d)", d.Name, d.ID)
	}
	return strings.Join(labels, ", ")
}

// resolveDevices resolves every --device-ids token and confirms they all
// share one template — a shipment is firmware for one template, so mixed
// templates across --device-ids is a hard error.
func resolveDevices(client *api.Client, raw string) ([]api.Device, int32, error) {
	tokens, err := splitDeviceTokens(raw)
	if err != nil {
		return nil, 0, err
	}

	devices := make([]api.Device, 0, len(tokens))
	for _, token := range tokens {
		d, err := resolveDeviceToken(client, token)
		if err != nil {
			return nil, 0, fmt.Errorf("resolve device %q: %w", token, err)
		}
		devices = append(devices, *d)
	}

	templateID := devices[0].TemplateID
	for _, d := range devices[1:] {
		if d.TemplateID != templateID {
			return nil, 0, fmt.Errorf("--device-ids span different templates (device %d is template %d, device %d is template %d) — a shipment targets one template, split this into separate shipments", devices[0].ID, templateID, d.ID, d.TemplateID)
		}
	}
	return devices, templateID, nil
}

func deviceLabel(d api.Device) string {
	return fmt.Sprintf("%s, id %d", d.Name, d.ID)
}

func deviceListLabel(devices []api.Device) string {
	if len(devices) == 1 {
		return deviceLabel(devices[0])
	}
	names := make([]string, len(devices))
	for i, d := range devices {
		names[i] = d.Name
	}
	return strings.Join(names, ", ")
}

func autoShipmentName(devices []api.Device, file string) string {
	ts := time.Now().Format("2006-01-02 15:04")
	fw := filepath.Base(file)
	if len(devices) == 1 {
		return fmt.Sprintf("%s · %s · %s", devices[0].Name, fw, ts)
	}
	return fmt.Sprintf("template %d · %s · %s", devices[0].TemplateID, fw, ts)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// waitForShipment polls GET /shipment until it reaches a terminal status
// (FINISH/CANCEL) or waitTimeout elapses, printing progress along the way.
// There is no per-device status endpoint — only the aggregate
// shipmentProgress counters — so progress is reported from those.
func waitForShipment(client *api.Client, shipmentID int64, deviceCount int, devices []api.Device, waitTimeout time.Duration, verbose bool) error {
	deadline := time.Now().Add(waitTimeout)
	var lastStage string

	for {
		s, err := client.GetShipment(shipmentID, flagOrgID)
		if err != nil {
			return err
		}
		p := s.ShipmentProgress

		if deviceCount == 1 {
			stage := singleDeviceStage(p)
			if stage != lastStage {
				fmt.Printf("  %s: %s\n", devices[0].Name, stage)
				lastStage = stage
			}
		} else if verbose {
			fmt.Printf("\rstarted=%d requestSent=%d firmwareRequested=%d firmwareUploaded=%d success=%d failures=%d        ",
				p.Started, p.RequestSent, p.FirmwareRequested, p.FirmwareUploaded, p.Success, p.Failures())
		} else {
			pending := int32(deviceCount) - p.Success - p.Failures()
			fmt.Printf("\rRolling out... %d/%d updated, %d failed, %d pending        ", p.Success, deviceCount, p.Failures(), pending)
		}

		if s.Terminal() {
			if deviceCount > 1 {
				fmt.Println()
			}
			if p.Success >= int32(deviceCount) {
				fmt.Printf("\n✓ Shipment %d complete — firmware applied successfully\n", s.ID)
				return nil
			}
			fmt.Printf("\n✗ Shipment %d finished with failures (%d/%d succeeded) — see `blynk shipment get --id %d` for details\n", s.ID, p.Success, deviceCount, s.ID)
			return fmt.Errorf("shipment %d did not fully succeed", s.ID)
		}

		if time.Now().After(deadline) {
			if deviceCount > 1 {
				fmt.Println()
			}
			return fmt.Errorf("timed out waiting for shipment %d (last status: %s)", s.ID, s.Status)
		}

		time.Sleep(2 * time.Second)
	}
}

// singleDeviceStage infers a human-readable stage label for a one-device
// shipment from which aggregate counters have been incremented.
func singleDeviceStage(p api.ShipmentProgress) string {
	switch {
	case p.Success > 0:
		return "applied ✓"
	case p.Rollback > 0:
		return "rolled back ✗"
	case p.FirmwareVersionMismatch > 0:
		return "firmware version mismatch ✗"
	case p.FirmwareTypeMismatch > 0:
		return "firmware type mismatch ✗"
	case p.DownloadLimitReached > 0:
		return "download limit reached ✗"
	case p.UploadFailure > 0:
		return "upload failure ✗"
	case p.FirmwareUploadedToMobile > 0:
		return "firmware uploaded to mobile relay"
	case p.FirmwareUploaded > 0:
		return "firmware uploaded to device"
	case p.FirmwareRequested > 0:
		return "firmware requested"
	case p.RequestSent > 0:
		return "notified"
	case p.Started > 0:
		return "started"
	default:
		return "initiated"
	}
}
