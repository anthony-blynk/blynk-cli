package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

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

// deviceListItem is what `device list` actually renders: a Device with its
// auth token redacted by default (mirroring `device get`/`profile show`),
// plus a resolved template name and an online status that's only populated
// (and only serialized, via omitempty) when --online was passed.
type deviceListItem struct {
	api.Device
	TemplateName string `json:"templateName,omitempty"`
	Online       string `json:"online,omitempty"`
}

func deviceListCmd() *cobra.Command {
	var includeSubOrgDevices bool
	var checkOnline bool
	var reveal bool
	var templateFlag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List devices",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := requireClient()
			if err != nil {
				return err
			}

			var templateFilterID int32
			filtering := templateFlag != ""
			if filtering {
				tpl, err := resolveTemplateToken(client, templateFlag)
				if err != nil {
					return fmt.Errorf("resolve template %q: %w", templateFlag, err)
				}
				templateFilterID = tpl.ID
			}

			// Filtering by template is client-side (no server-side
			// template filter on this endpoint), so a partial page would
			// give a misleadingly incomplete result — always fetch every
			// page in that case, regardless of --all.
			fetchAll := flagAll || filtering

			var devices []api.Device
			seen := 0
			page := flagPage
			for {
				batch, total, err := client.ListDevices(flagOrgID, includeSubOrgDevices, page, flagSize)
				if err != nil {
					return err
				}
				seen += len(batch)
				for _, d := range batch {
					if !filtering || d.TemplateID == templateFilterID {
						devices = append(devices, d)
					}
				}
				if !fetchAll || len(batch) == 0 || seen >= int(total) {
					break
				}
				page++
			}

			tplNames := templateNames(client, devices)

			var onlineStatuses []string
			if checkOnline {
				onlineStatuses = fetchOnlineStatuses(client, devices)
			}

			headers := []string{"ID", "NAME", "TEMPLATE", "FIRMWARE", "BOARD"}
			if checkOnline {
				headers = append(headers, "ONLINE")
			}
			t := &output.Table{Headers: headers}

			items := make([]deviceListItem, len(devices))
			for i, d := range devices {
				name := tplNames[d.TemplateID]
				item := deviceListItem{Device: d, TemplateName: name}
				if !reveal {
					item.Token = ""
				}

				version, board := "", ""
				if d.HardwareInfo != nil {
					version = d.HardwareInfo.Version
					board = d.HardwareInfo.BoardType
				}
				row := []string{
					strconv.FormatInt(d.ID, 10),
					d.Name,
					templateLabel(d.TemplateID, name),
					version,
					board,
				}
				if checkOnline {
					item.Online = onlineStatuses[i]
					row = append(row, onlineStatuses[i])
				}

				items[i] = item
				t.Rows = append(t.Rows, row)
			}
			return output.Render(os.Stdout, flagOutput, items, t)
		},
	}

	cmd.Flags().BoolVar(&includeSubOrgDevices, "include-sub-org-devices", false, "Include devices from sub-organizations")
	cmd.Flags().BoolVar(&checkOnline, "online", false, "Also check and show each device's live online status (one extra request per device)")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Show each device's auth token")
	cmd.Flags().StringVar(&templateFlag, "template", "", "Only list devices belonging to this template (id or name)")
	return cmd
}

// fetchOnlineStatuses checks IsOnline for every device concurrently (bounded
// so a large --all listing doesn't fire hundreds of requests at once), and
// returns "online"/"offline"/"?" (on a per-device error) in the same order
// as devices — one bad response shouldn't blank out the whole listing.
func fetchOnlineStatuses(client *api.Client, devices []api.Device) []string {
	const maxConcurrency = 8
	statuses := make([]string, len(devices))
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for i, d := range devices {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, deviceID int64) {
			defer wg.Done()
			defer func() { <-sem }()
			online, err := client.IsOnline(deviceID)
			switch {
			case err != nil:
				statuses[i] = "?"
			case online:
				statuses[i] = "online"
			default:
				statuses[i] = "offline"
			}
		}(i, d.ID)
	}
	wg.Wait()
	return statuses
}

// templateNames resolves a display name for every distinct TemplateID
// across devices — deduped and fetched concurrently (bounded), since the
// number of distinct templates is normally far smaller than the number of
// devices. A template whose lookup fails is simply left out of the map;
// callers fall back to showing the numeric id via templateLabel.
func templateNames(client *api.Client, devices []api.Device) map[int32]string {
	ids := map[int32]struct{}{}
	for _, d := range devices {
		ids[d.TemplateID] = struct{}{}
	}

	names := make(map[int32]string, len(ids))
	var mu sync.Mutex
	var wg sync.WaitGroup
	const maxConcurrency = 8
	sem := make(chan struct{}, maxConcurrency)

	for id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id int32) {
			defer wg.Done()
			defer func() { <-sem }()
			tpl, err := client.GetTemplate(id)
			if err != nil || tpl.Name == "" {
				return
			}
			mu.Lock()
			names[id] = tpl.Name
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return names
}

// templateLabel renders a template id with its resolved name when known,
// e.g. "Linux Agent (345660)", falling back to just the numeric id.
func templateLabel(id int32, name string) string {
	if name == "" {
		return strconv.FormatInt(int64(id), 10)
	}
	return fmt.Sprintf("%s (%d)", name, id)
}

// resolveTemplateToken resolves a --template value to a Template: a
// numeric token is looked up directly; anything else is resolved by
// listing every template (there's no template search-by-name endpoint)
// and matching by name, case-insensitive, preferring an exact match over
// a substring one — mirroring resolveDeviceToken's approach.
func resolveTemplateToken(client *api.Client, token string) (*api.Template, error) {
	if id, err := strconv.ParseInt(token, 10, 32); err == nil {
		return client.GetTemplate(int32(id))
	}

	var all []api.Template
	seen := 0
	page := 0
	for {
		batch, total, err := client.ListTemplates(flagOrgID, page, 200)
		if err != nil {
			return nil, fmt.Errorf("list templates: %w", err)
		}
		all = append(all, batch...)
		seen += len(batch)
		if len(batch) == 0 || seen >= int(total) {
			break
		}
		page++
	}

	var exact, substr []api.Template
	lower := strings.ToLower(token)
	for _, tpl := range all {
		lname := strings.ToLower(tpl.Name)
		switch {
		case lname == lower:
			exact = append(exact, tpl)
		case strings.Contains(lname, lower):
			substr = append(substr, tpl)
		}
	}
	switch {
	case len(exact) == 1:
		return &exact[0], nil
	case len(exact) > 1:
		return nil, fmt.Errorf("ambiguous template name %q, candidates: %s", token, templateCandidates(exact))
	case len(substr) == 1:
		return &substr[0], nil
	case len(substr) > 1:
		return nil, fmt.Errorf("ambiguous template name %q, candidates: %s", token, templateCandidates(substr))
	default:
		return nil, fmt.Errorf("no template found matching %q", token)
	}
}

func templateCandidates(templates []api.Template) string {
	labels := make([]string, len(templates))
	for i, tpl := range templates {
		labels[i] = fmt.Sprintf("%s (id %d)", tpl.Name, tpl.ID)
	}
	return strings.Join(labels, ", ")
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
				fmt.Println(onlineLabel(online))
				return nil
			}

			// Best-effort: a template-name lookup failure shouldn't sink
			// the whole command, just fall back to showing the numeric id.
			var templateName string
			if tpl, err := client.GetTemplate(d.TemplateID); err == nil {
				templateName = tpl.Name
			}

			display := *d
			if !reveal {
				display.Token = "" // device auth credential — never print by default
			}

			detail := struct {
				api.Device
				TemplateName string `json:"templateName,omitempty"`
				Online       string `json:"online"`
			}{Device: display, TemplateName: templateName, Online: onlineLabel(online)}

			return output.Render(os.Stdout, flagOutput, &detail, deviceTable(&display, online, reveal, templateName))
		},
	}

	cmd.Flags().StringVar(&idFlag, "id", "", "Device ID or name (required)")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Show the device's auth token")
	return cmd
}

func onlineLabel(online bool) string {
	if online {
		return "online"
	}
	return "offline"
}

func deviceTable(d *api.Device, online bool, reveal bool, templateName string) *output.Table {
	t := &output.Table{Headers: []string{"FIELD", "VALUE"}}
	t.Rows = append(t.Rows,
		[]string{"id", strconv.FormatInt(d.ID, 10)},
		[]string{"name", d.Name},
		[]string{"template", templateLabel(d.TemplateID, templateName)},
		[]string{"online", onlineLabel(online)},
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
