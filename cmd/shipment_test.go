package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/anthony-blynk/blynk-cli/internal/api"
)

func TestSplitDeviceTokens(t *testing.T) {
	cases := []struct {
		raw     string
		want    []string
		wantErr bool
	}{
		{"123,456", []string{"123", "456"}, false},
		{" 123 , AntsDemoTest ", []string{"123", "AntsDemoTest"}, false},
		{"123", []string{"123"}, false},
		{"", nil, true},
		{"  ,  ", nil, true},
	}
	for _, tc := range cases {
		got, err := splitDeviceTokens(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("splitDeviceTokens(%q): expected error, got %v", tc.raw, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitDeviceTokens(%q): unexpected error: %v", tc.raw, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("splitDeviceTokens(%q) = %v, want %v", tc.raw, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("splitDeviceTokens(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		}
	}
}

func TestDeviceIDs32(t *testing.T) {
	devices := []api.Device{{ID: 1}, {ID: 2}, {ID: 569482}}
	got := deviceIDs32(devices)
	want := []int32{1, 2, 569482}
	if len(got) != len(want) {
		t.Fatalf("deviceIDs32 = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("deviceIDs32 = %v, want %v", got, want)
		}
	}
}

func TestDeviceLabel(t *testing.T) {
	d := api.Device{ID: 569482, Name: "Nvidia ORIN"}
	want := "Nvidia ORIN, id 569482"
	if got := deviceLabel(d); got != want {
		t.Errorf("deviceLabel = %q, want %q", got, want)
	}
}

func TestDeviceListLabelSingle(t *testing.T) {
	devices := []api.Device{{ID: 1, Name: "boiler-3"}}
	if got, want := deviceListLabel(devices), "boiler-3, id 1"; got != want {
		t.Errorf("deviceListLabel = %q, want %q", got, want)
	}
}

func TestDeviceListLabelMultiple(t *testing.T) {
	devices := []api.Device{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}
	if got, want := deviceListLabel(devices), "a, b"; got != want {
		t.Errorf("deviceListLabel = %q, want %q", got, want)
	}
}

func TestAutoShipmentNameSingleDevice(t *testing.T) {
	devices := []api.Device{{ID: 1, Name: "boiler-3"}}
	got := autoShipmentName(devices, "fw-2.3.1.bin")
	re := regexp.MustCompile(`^boiler-3 · fw-2\.3\.1\.bin · \d{4}-\d{2}-\d{2} \d{2}:\d{2}$`)
	if !re.MatchString(got) {
		t.Errorf("autoShipmentName (single) = %q, doesn't match expected pattern", got)
	}
}

func TestAutoShipmentNameMultipleDevices(t *testing.T) {
	devices := []api.Device{{ID: 1, Name: "a", TemplateID: 345660}, {ID: 2, Name: "b", TemplateID: 345660}}
	got := autoShipmentName(devices, "fw.bin")
	re := regexp.MustCompile(`^template 345660 · fw\.bin · \d{4}-\d{2}-\d{2} \d{2}:\d{2}$`)
	if !re.MatchString(got) {
		t.Errorf("autoShipmentName (multi) = %q, doesn't match expected pattern", got)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{3181, "3.1 KiB"}, // the real docker-compose.yml size from live testing
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1258291, "1.2 MiB"},
	}
	for _, tc := range cases {
		if got := humanBytes(tc.n); got != tc.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestSingleDeviceStage(t *testing.T) {
	cases := []struct {
		name string
		p    api.ShipmentProgress
		want string
	}{
		{"nothing yet", api.ShipmentProgress{}, "initiated"},
		{"started only", api.ShipmentProgress{Started: 1}, "started"},
		{"notified", api.ShipmentProgress{Started: 1, RequestSent: 1}, "notified"},
		{"firmware requested", api.ShipmentProgress{Started: 1, RequestSent: 1, FirmwareRequested: 1}, "firmware requested"},
		{"firmware uploaded", api.ShipmentProgress{Started: 1, RequestSent: 1, FirmwareRequested: 1, FirmwareUploaded: 1}, "firmware uploaded to device"},
		{"uploaded to mobile relay", api.ShipmentProgress{FirmwareUploadedToMobile: 1}, "firmware uploaded to mobile relay"},
		{"success wins over everything", api.ShipmentProgress{Started: 1, RequestSent: 1, FirmwareUploaded: 1, Success: 1}, "applied ✓"},
		{"upload failure", api.ShipmentProgress{UploadFailure: 1}, "upload failure ✗"},
		{"firmware type mismatch", api.ShipmentProgress{FirmwareTypeMismatch: 1}, "firmware type mismatch ✗"},
		{"download limit reached", api.ShipmentProgress{DownloadLimitReached: 1}, "download limit reached ✗"},
		{"firmware version mismatch", api.ShipmentProgress{FirmwareVersionMismatch: 1}, "firmware version mismatch ✗"},
		{"rollback", api.ShipmentProgress{Rollback: 1}, "rolled back ✗"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := singleDeviceStage(tc.p); got != tc.want {
				t.Errorf("singleDeviceStage(%+v) = %q, want %q", tc.p, got, tc.want)
			}
		})
	}
}

// mockDeviceServer serves GET /organization/device and
// GET /organization/search/devices from an in-memory device list, letting
// resolveDevices/resolveDeviceToken be tested without a real server.
func mockDeviceServer(t *testing.T, devices []api.Device) *api.Client {
	t.Helper()
	byID := map[string]api.Device{}
	for _, d := range devices {
		byID[strconv.FormatInt(d.ID, 10)] = d
	}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/organization/device":
			d, ok := byID[r.URL.Query().Get("deviceId")]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"device not found"}`))
				return
			}
			json.NewEncoder(w).Encode(d)
		case "/api/v1/organization/search/devices":
			query := r.URL.Query().Get("query")
			var matches []api.Device
			for _, d := range devices {
				if strings.Contains(strings.ToLower(d.Name), strings.ToLower(query)) {
					matches = append(matches, d)
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"content": matches, "totalElements": len(matches)})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	c := api.NewClient(srv.Listener.Addr().String(), "faketoken")
	c.HTTP = srv.Client()
	return c
}

func TestResolveDevicesNumericID(t *testing.T) {
	client := mockDeviceServer(t, []api.Device{{ID: 569482, Name: "Nvidia ORIN", TemplateID: 345660}})
	devices, templateID, err := resolveDevices(client, "569482")
	if err != nil {
		t.Fatalf("resolveDevices: %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "Nvidia ORIN" || templateID != 345660 {
		t.Errorf("devices=%+v templateID=%d", devices, templateID)
	}
}

func TestResolveDevicesByExactName(t *testing.T) {
	client := mockDeviceServer(t, []api.Device{
		{ID: 1, Name: "AntsDemoTest", TemplateID: 887402},
		{ID: 2, Name: "AntsDemoTestExtra", TemplateID: 887402},
	})
	devices, _, err := resolveDevices(client, "AntsDemoTest")
	if err != nil {
		t.Fatalf("resolveDevices: %v", err)
	}
	if len(devices) != 1 || devices[0].ID != 1 {
		t.Errorf("expected the exact-name match (id 1), got %+v", devices)
	}
}

func TestResolveDevicesAmbiguousNameIsError(t *testing.T) {
	client := mockDeviceServer(t, []api.Device{
		{ID: 1, Name: "device-a"},
		{ID: 2, Name: "device-b"},
	})
	if _, _, err := resolveDevices(client, "device"); err == nil {
		t.Fatal("expected an ambiguous-match error")
	}
}

func TestResolveDevicesNoMatchIsError(t *testing.T) {
	client := mockDeviceServer(t, []api.Device{{ID: 1, Name: "device-a"}})
	if _, _, err := resolveDevices(client, "nonexistent"); err == nil {
		t.Fatal("expected a no-match error")
	}
}

func TestResolveDevicesMixedTemplatesIsError(t *testing.T) {
	client := mockDeviceServer(t, []api.Device{
		{ID: 1, Name: "a", TemplateID: 100},
		{ID: 2, Name: "b", TemplateID: 200},
	})
	if _, _, err := resolveDevices(client, "1,2"); err == nil {
		t.Fatal("expected a mixed-template error")
	}
}
