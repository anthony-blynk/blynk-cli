package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/anthony-blynk/blynk-cli/internal/api"
	"github.com/anthony-blynk/blynk-cli/internal/ble"
	"github.com/spf13/cobra"
)

func deviceProvisionCmd() *cobra.Command {
	var templateFlag, nameFlag, deviceFlag, bleName, wifiSSID, wifiPass string
	var scanTimeout, connectTimeout, onlineTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Create a device and pair it over Bluetooth, in place of the mobile app",
		Long: `Finds a device advertising Blynk BLE provisioning, creates a Blynk device
for it (or reuses one with --device), hands it the auth token and server
over Bluetooth, and waits for it to come online. The token is never printed.

  blynk device provision --template "Modbus Gateway"
  blynk device provision --template 349155 --ble-name F8XQ --name "Pump room gateway"
  blynk device provision --device 576285          # retry / re-pair an existing device
  blynk device provision --template "T-Vending" --wifi-ssid MyNet   # ESP32: prompts for the password`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (templateFlag == "") == (deviceFlag == "") {
				return errors.New("pass exactly one of --template (create a new device) or --device (pair an existing one)")
			}
			if deviceFlag != "" && nameFlag != "" {
				return errors.New("--name only applies when creating a device with --template")
			}

			client, err := requireClient()
			if err != nil {
				return err
			}

			// Resolve the cloud side before touching the radio, so a typo
			// fails fast.
			var existing *api.Device
			var tpl *api.Template
			if deviceFlag != "" {
				d, err := resolveDeviceToken(client, deviceFlag)
				if err != nil {
					return fmt.Errorf("resolve device %q: %w", deviceFlag, err)
				}
				if existing, err = client.GetDevice(d.ID); err != nil {
					return err
				}
				if tpl, err = client.GetTemplate(existing.TemplateID); err != nil {
					return fmt.Errorf("get template %d: %w", existing.TemplateID, err)
				}
			} else if tpl, err = resolveTemplateToken(client, templateFlag); err != nil {
				return fmt.Errorf("resolve template %q: %w", templateFlag, err)
			}
			if len(tpl.TemplateIDs) == 0 {
				return fmt.Errorf("template %s has no alphanumeric template id (TMPL...)", templateLabel(tpl.ID, tpl.Name))
			}
			tmplID := tpl.TemplateIDs[0]

			if wifiSSID != "" && !cmd.Flags().Changed("wifi-pass") {
				if wifiPass, err = promptSecret(fmt.Sprintf("Wi-Fi password for %q (empty for an open network)", wifiSSID)); err != nil {
					return err
				}
			}

			fmt.Println("Scanning for devices in provisioning mode...")
			scanCtx, cancel := context.WithTimeout(context.Background(), scanTimeout)
			found, err := ble.Scan(scanCtx, bleName, 2*time.Second)
			cancel()
			if err != nil {
				return err
			}
			p, err := pickPeripheral(found, bleName, scanTimeout)
			if err != nil {
				return err
			}

			fmt.Printf("Connecting to %q (%s)...\n", p.Name, p.Address)
			sess, err := ble.Dial(p, connectTimeout)
			if err != nil {
				return err
			}
			defer sess.Close()

			infoCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			info, err := sess.Info(infoCtx)
			cancel()
			if err != nil {
				return fmt.Errorf("read device info: %w", err)
			}
			fmt.Printf("✓ Connected: firmware %s %s, template %s\n", info.FwType, info.FwVer, info.TmplID)

			// Fixed firmware has its template baked in; pairing it with a
			// device on another template would never work.
			if !info.Dynamic() && info.TmplID != "" && !slices.Contains(tpl.TemplateIDs, info.TmplID) {
				return fmt.Errorf("%q runs firmware for template %s, but %s is %s", p.Name, info.TmplID, templateLabel(tpl.ID, tpl.Name), strings.Join(tpl.TemplateIDs, "/"))
			}

			name := nameFlag
			if name == "" {
				name = defaultProvisionName(p.Name)
			}
			prompt := fmt.Sprintf("About to create device %q on template %s and pair %q with it. Continue?", name, templateLabel(tpl.ID, tpl.Name), p.Name)
			if existing != nil {
				prompt = fmt.Sprintf("About to pair %q with existing device %q (id %d). Continue?", p.Name, existing.Name, existing.ID)
			}
			if !confirm(prompt) {
				fmt.Println("Cancelled.")
				return nil
			}

			d := existing
			if d == nil {
				if d, err = client.CreateDevice(api.CreateDeviceRequest{TemplateID: tmplID, Name: name, OrgID: flagOrgID}); err != nil {
					return fmt.Errorf("create device: %w", err)
				}
				fmt.Printf("✓ Device %d %q created\n", d.ID, d.Name)
			}
			if d.Token == "" {
				return fmt.Errorf("device %d came back without an auth token", d.ID)
			}
			// From here on a failure leaves a device that exists but isn't
			// paired; say how to retry without creating another one.
			retry := func(err error) error {
				return fmt.Errorf("%w\ndevice %d exists but isn't paired; retry with `blynk device provision --device %d`", err, d.ID, d.ID)
			}

			cfg := ble.Config{Token: d.Token, Host: client.Server, WiFiSSID: wifiSSID, WiFiPass: wifiPass}
			if info.Dynamic() {
				cfg.TemplateID = tmplID
			}
			// Joining Wi-Fi or a cellular network can take a while.
			pairCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := sess.Configure(pairCtx, cfg); err != nil {
				return retry(fmt.Errorf("send credentials: %w", err))
			}
			fmt.Printf("✓ Sent credentials (server %s)\n", client.Server)
			err = sess.Connect(pairCtx, func(stage string) {
				if label, ok := provisionStageLabels[stage]; ok {
					fmt.Printf("  %s\n", label)
				}
			})
			if err != nil {
				return retry(err)
			}
			sess.Close()
			fmt.Println("  device reports connected")

			if err := waitOnline(client, d.ID, onlineTimeout); err != nil {
				return retry(err)
			}
			fmt.Printf("✓ Device %d %q online\n", d.ID, d.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&templateFlag, "template", "", "Create a new device on this template (id or name)")
	cmd.Flags().StringVar(&deviceFlag, "device", "", "Pair an existing device instead of creating one (id or name)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "Name for the new device (default: the advertised name without \"Blynk \")")
	cmd.Flags().StringVar(&bleName, "ble-name", "", "Only pair a device whose advertised name contains this, e.g. its F8XQ suffix")
	cmd.Flags().StringVar(&wifiSSID, "wifi-ssid", "", "Wi-Fi network for the device to join (Wi-Fi boards such as ESP32)")
	cmd.Flags().StringVar(&wifiPass, "wifi-pass", "", "Wi-Fi password (prompted for if --wifi-ssid is set and this isn't)")
	cmd.Flags().DurationVar(&scanTimeout, "scan-timeout", 30*time.Second, "How long to look for an advertising device")
	cmd.Flags().DurationVar(&connectTimeout, "connect-timeout", 30*time.Second, "How long to wait for the Bluetooth link (Windows can take ~15 s)")
	cmd.Flags().DurationVar(&onlineTimeout, "online-timeout", time.Minute, "How long to wait for the device to show online in Blynk")
	return cmd
}

var provisionStageLabels = map[string]string{
	"connecting":       "connecting...",
	"connecting_net":   "joining the network",
	"connecting_cloud": "connecting to Blynk",
}

// pickPeripheral requires exactly one advertising device, so credentials
// never go to the wrong one when several are in pairing mode nearby.
func pickPeripheral(found []ble.Peripheral, bleName string, scanTimeout time.Duration) (ble.Peripheral, error) {
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		if bleName != "" {
			return ble.Peripheral{}, fmt.Errorf("no device in provisioning mode matching %q found within %s", bleName, scanTimeout)
		}
		return ble.Peripheral{}, fmt.Errorf("no device in provisioning mode found within %s; is it advertising, and is Bluetooth on?", scanTimeout)
	default:
		labels := make([]string, len(found))
		for i, p := range found {
			labels[i] = fmt.Sprintf("%q (%s)", p.Name, p.Address)
		}
		return ble.Peripheral{}, fmt.Errorf("%d devices are in provisioning mode: %s; pass --ble-name to pick one", len(found), strings.Join(labels, ", "))
	}
}

// defaultProvisionName turns an advertised name like "Blynk Modbus
// Gateway-F8XQ" into a device name, "Modbus Gateway-F8XQ".
func defaultProvisionName(advertised string) string {
	if name := strings.TrimPrefix(advertised, "Blynk "); name != "" {
		return name
	}
	return advertised
}

// waitOnline polls until Blynk sees the device connected. The device's own
// "connected" only means it has started connecting with the new token.
func waitOnline(client *api.Client, deviceID int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		online, err := client.IsOnline(deviceID)
		if err == nil && online {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("check online status: %w", err)
			}
			return fmt.Errorf("device didn't show online in Blynk within %s", timeout)
		}
		time.Sleep(2 * time.Second)
	}
}
