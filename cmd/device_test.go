package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/anthony-blynk/blynk-cli/internal/api"
)

func TestDeviceTableHidesTokenByDefault(t *testing.T) {
	d := &api.Device{ID: 1, Name: "boiler-3", Token: "super-secret-device-token"}
	tbl := deviceTable(d, true, false, "")
	for _, row := range tbl.Rows {
		for _, cell := range row {
			if strings.Contains(cell, "super-secret-device-token") {
				t.Fatalf("device token leaked into table output without --reveal: %v", row)
			}
		}
	}
}

func TestDeviceTableRevealsTokenWhenAsked(t *testing.T) {
	d := &api.Device{ID: 1, Name: "boiler-3", Token: "super-secret-device-token"}
	tbl := deviceTable(d, true, true, "")
	found := false
	for _, row := range tbl.Rows {
		if len(row) == 2 && row[0] == "token" && row[1] == "super-secret-device-token" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a token row when --reveal is set")
	}
}

func TestDeviceTableOnlineStatus(t *testing.T) {
	cases := []struct {
		online bool
		want   string
	}{
		{true, "online"},
		{false, "offline"},
	}
	for _, tc := range cases {
		tbl := deviceTable(&api.Device{ID: 1}, tc.online, false, "")
		found := false
		for _, row := range tbl.Rows {
			if len(row) == 2 && row[0] == "online" && row[1] == tc.want {
				found = true
			}
		}
		if !found {
			t.Errorf("online=%v: expected row [online %q] in %v", tc.online, tc.want, tbl.Rows)
		}
	}
}

func TestDeviceTableOmitsMissingHardwareInfo(t *testing.T) {
	tbl := deviceTable(&api.Device{ID: 1, Name: "n"}, false, false, "")
	for _, row := range tbl.Rows {
		if len(row) > 0 && row[0] == "firmware_version" {
			t.Errorf("firmware_version row present with nil HardwareInfo: %v", tbl.Rows)
		}
	}
}

func TestDeviceTableIncludesFirmwareVersion(t *testing.T) {
	d := &api.Device{
		ID:   1,
		Name: "n",
		HardwareInfo: &api.DeviceHardwareInfo{
			Version:   "2.3.1",
			BoardType: "Other",
		},
	}
	tbl := deviceTable(d, true, false, "")
	found := false
	for _, row := range tbl.Rows {
		if len(row) == 2 && row[0] == "firmware_version" && row[1] == "2.3.1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected firmware_version row = 2.3.1, got %v", tbl.Rows)
	}
}

func TestDeviceTableShowsTemplateNameWhenResolved(t *testing.T) {
	tbl := deviceTable(&api.Device{ID: 1, TemplateID: 345660}, false, false, "Linux Agent")
	found := false
	for _, row := range tbl.Rows {
		if len(row) == 2 && row[0] == "template" && row[1] == "Linux Agent (345660)" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected template row = 'Linux Agent (345660)', got %v", tbl.Rows)
	}
}

func TestDeviceTableFallsBackToTemplateIDWhenNameUnresolved(t *testing.T) {
	tbl := deviceTable(&api.Device{ID: 1, TemplateID: 345660}, false, false, "")
	found := false
	for _, row := range tbl.Rows {
		if len(row) == 2 && row[0] == "template" && row[1] == "345660" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected template row = '345660' (numeric fallback), got %v", tbl.Rows)
	}
}

func TestTemplateLabel(t *testing.T) {
	if got, want := templateLabel(345660, "Linux Agent"), "Linux Agent (345660)"; got != want {
		t.Errorf("templateLabel(with name) = %q, want %q", got, want)
	}
	if got, want := templateLabel(345660, ""), "345660"; got != want {
		t.Errorf("templateLabel(no name) = %q, want %q", got, want)
	}
}

func TestDeviceListItemJSONOmitsOnlineWhenNotChecked(t *testing.T) {
	item := deviceListItem{Device: api.Device{ID: 1, Name: "n"}}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"online"`) {
		t.Errorf("expected no 'online' field when --online wasn't used: %s", data)
	}
}

func TestDeviceListItemJSONIncludesOnlineWhenChecked(t *testing.T) {
	item := deviceListItem{Device: api.Device{ID: 1}, Online: "online"}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"online":"online"`) {
		t.Errorf("expected an online field: %s", data)
	}
}

// mockOnlineServer serves GET /organization/device/online from an
// in-memory map keyed by device id; ids in failFor return a 500 instead,
// to exercise fetchOnlineStatuses' per-device error handling.
func mockOnlineServer(t *testing.T, online map[int64]bool, failFor map[int64]bool) *api.Client {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.URL.Query().Get("deviceId"), 10, 64)
		if failFor[id] {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"message":"boom"}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"connected": online[id]})
	}))
	t.Cleanup(srv.Close)
	c := api.NewClient(srv.Listener.Addr().String(), "faketoken")
	c.HTTP = srv.Client()
	return c
}

func TestFetchOnlineStatusesOrderingAndPerDeviceErrors(t *testing.T) {
	devices := []api.Device{{ID: 1}, {ID: 2}, {ID: 3}}
	client := mockOnlineServer(t,
		map[int64]bool{1: true, 2: false, 3: true},
		map[int64]bool{3: true}, // device 3's request errors
	)

	got := fetchOnlineStatuses(client, devices)
	want := []string{"online", "offline", "?"}
	if len(got) != len(want) {
		t.Fatalf("fetchOnlineStatuses = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("fetchOnlineStatuses[%d] = %q, want %q (order must match input devices despite concurrency)", i, got[i], want[i])
		}
	}
}

func TestFetchOnlineStatusesManyDevicesPreservesOrder(t *testing.T) {
	const n = 30
	devices := make([]api.Device, n)
	online := map[int64]bool{}
	want := make([]string, n)
	for i := range devices {
		id := int64(i + 1)
		devices[i] = api.Device{ID: id}
		// Alternate online/offline so a mis-ordered result would be caught.
		online[id] = i%2 == 0
		if online[id] {
			want[i] = "online"
		} else {
			want[i] = "offline"
		}
	}
	client := mockOnlineServer(t, online, nil)

	got := fetchOnlineStatuses(client, devices)
	if len(got) != n {
		t.Fatalf("len = %d, want %d", len(got), n)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// mockTemplateServer serves GET /organization/template (by id) and
// GET /organization/templates (list, single page) from an in-memory list.
func mockTemplateServer(t *testing.T, templates []api.Template) *api.Client {
	t.Helper()
	byID := map[string]api.Template{}
	for _, tpl := range templates {
		byID[strconv.FormatInt(int64(tpl.ID), 10)] = tpl
	}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/organization/template":
			tpl, ok := byID[r.URL.Query().Get("templateId")]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"template not found"}`))
				return
			}
			json.NewEncoder(w).Encode(tpl)
		case "/api/v1/organization/templates":
			json.NewEncoder(w).Encode(map[string]any{
				"content":       templates,
				"totalElements": len(templates),
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := api.NewClient(srv.Listener.Addr().String(), "faketoken")
	c.HTTP = srv.Client()
	return c
}

func TestResolveTemplateTokenNumericID(t *testing.T) {
	client := mockTemplateServer(t, []api.Template{{ID: 345660, Name: "Linux Agent"}})
	tpl, err := resolveTemplateToken(client, "345660")
	if err != nil {
		t.Fatalf("resolveTemplateToken: %v", err)
	}
	if tpl.Name != "Linux Agent" {
		t.Errorf("Name = %q, want Linux Agent", tpl.Name)
	}
}

func TestResolveTemplateTokenByExactName(t *testing.T) {
	client := mockTemplateServer(t, []api.Template{
		{ID: 1, Name: "Linux Agent"},
		{ID: 2, Name: "Linux Agent Extra"},
	})
	tpl, err := resolveTemplateToken(client, "Linux Agent")
	if err != nil {
		t.Fatalf("resolveTemplateToken: %v", err)
	}
	if tpl.ID != 1 {
		t.Errorf("expected the exact-name match (id 1), got %+v", tpl)
	}
}

func TestResolveTemplateTokenAmbiguousIsError(t *testing.T) {
	client := mockTemplateServer(t, []api.Template{
		{ID: 1, Name: "template-a"},
		{ID: 2, Name: "template-b"},
	})
	if _, err := resolveTemplateToken(client, "template"); err == nil {
		t.Fatal("expected an ambiguous-match error")
	}
}

func TestResolveTemplateTokenNoMatchIsError(t *testing.T) {
	client := mockTemplateServer(t, []api.Template{{ID: 1, Name: "template-a"}})
	if _, err := resolveTemplateToken(client, "nonexistent"); err == nil {
		t.Fatal("expected a no-match error")
	}
}

func TestTemplateNamesDedupesAndFallsBackOnError(t *testing.T) {
	client := mockTemplateServer(t, []api.Template{
		{ID: 100, Name: "template-a"},
		// 200 deliberately absent from the server's template set, so its
		// lookup fails and it should just be missing from the result map.
	})
	devices := []api.Device{
		{ID: 1, TemplateID: 100},
		{ID: 2, TemplateID: 100}, // same template as device 1 — should only be fetched once conceptually
		{ID: 3, TemplateID: 200},
	}
	names := templateNames(client, devices)
	if names[100] != "template-a" {
		t.Errorf("names[100] = %q, want template-a", names[100])
	}
	if _, ok := names[200]; ok {
		t.Errorf("names[200] should be absent (lookup failure), got %q", names[200])
	}
}
