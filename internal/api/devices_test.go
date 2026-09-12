package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDeviceUnmarshal uses values actually seen live in this session
// (`shipment deploy --device-ids Nvidia --dry-run` resolved device 569482,
// name "Nvidia ORIN", template 345660) as a realistic fixture.
func TestDeviceUnmarshal(t *testing.T) {
	body := `{"id":569482,"name":"Nvidia ORIN","templateId":345660,"originalTemplateId":345660,"orgId":9740}`
	var d Device
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.ID != 569482 || d.Name != "Nvidia ORIN" || d.TemplateID != 345660 {
		t.Errorf("Device = %+v", d)
	}
}

func TestDeviceSearchResponseUnmarshalPaginatedWrapper(t *testing.T) {
	body := `{
		"content": [
			{"id": 1, "name": "AntsDemoTest", "templateId": 887402}
		],
		"totalElements": 1
	}`
	var resp devicePage
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TotalElements != 1 {
		t.Errorf("TotalElements = %d, want 1", resp.TotalElements)
	}
	if len(resp.Content) != 1 || resp.Content[0].Name != "AntsDemoTest" {
		t.Errorf("Content = %+v", resp.Content)
	}
}

func TestDeviceUnmarshalHardwareInfoAndLifecycleStatus(t *testing.T) {
	body := `{
		"id": 569482,
		"name": "Nvidia ORIN",
		"templateId": 345660,
		"hardwareInfo": {
			"version": "2.3.0-rc1",
			"fwType": "",
			"blynkVersion": "1.4.0",
			"boardType": "Other",
			"build": "42",
			"templateId": "TMPL0X9F"
		},
		"lifecycleStatus": {
			"eventId": 1,
			"name": "Active",
			"color": "green",
			"icon": "check"
		}
	}`
	var d Device
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.HardwareInfo == nil {
		t.Fatal("HardwareInfo is nil")
	}
	if d.HardwareInfo.Version != "2.3.0-rc1" || d.HardwareInfo.BoardType != "Other" {
		t.Errorf("HardwareInfo = %+v", d.HardwareInfo)
	}
	// The alphanumeric fw-embedded template id is a distinct field/value
	// space from the numeric Device.TemplateID above, despite the name.
	if d.HardwareInfo.TemplateID != "TMPL0X9F" {
		t.Errorf("HardwareInfo.TemplateID = %q, want TMPL0X9F", d.HardwareInfo.TemplateID)
	}
	if d.TemplateID != 345660 {
		t.Errorf("Device.TemplateID = %d, want 345660 (must not be confused with HardwareInfo.TemplateID)", d.TemplateID)
	}
	if d.LifecycleStatus == nil || d.LifecycleStatus.Name != "Active" {
		t.Errorf("LifecycleStatus = %+v", d.LifecycleStatus)
	}
}

func TestDeviceUnmarshalNilHardwareInfo(t *testing.T) {
	var d Device
	if err := json.Unmarshal([]byte(`{"id":1,"name":"n"}`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.HardwareInfo != nil {
		t.Errorf("HardwareInfo = %+v, want nil", d.HardwareInfo)
	}
	if d.LifecycleStatus != nil {
		t.Errorf("LifecycleStatus = %+v, want nil", d.LifecycleStatus)
	}
}

func mockAPIServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(srv.Listener.Addr().String(), "faketoken")
	c.HTTP = srv.Client()
	return c
}

func TestIsOnlineTrue(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/organization/device/online"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("deviceId"), "569482"; got != want {
			t.Errorf("deviceId = %q, want %q", got, want)
		}
		json.NewEncoder(w).Encode(map[string]bool{"connected": true})
	})

	online, err := client.IsOnline(569482)
	if err != nil {
		t.Fatalf("IsOnline: %v", err)
	}
	if !online {
		t.Error("IsOnline() = false, want true")
	}
}

func TestIsOnlineFalse(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"connected": false})
	})

	online, err := client.IsOnline(1)
	if err != nil {
		t.Fatalf("IsOnline: %v", err)
	}
	if online {
		t.Error("IsOnline() = true, want false")
	}
}

func TestListDevicesSendsQueryParamsAndReturnsTotal(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/organization/devices"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		q := r.URL.Query()
		if got, want := q.Get("orgId"), "9740"; got != want {
			t.Errorf("orgId = %q, want %q", got, want)
		}
		if got, want := q.Get("includeSubOrgDevices"), "true"; got != want {
			t.Errorf("includeSubOrgDevices = %q, want %q", got, want)
		}
		if got, want := q.Get("page"), "0"; got != want {
			t.Errorf("page = %q, want %q", got, want)
		}
		if got, want := q.Get("size"), "50"; got != want {
			t.Errorf("size = %q, want %q", got, want)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content":       []Device{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}},
			"totalElements": 2,
		})
	})

	devices, total, err := client.ListDevices(9740, true, 0, 50)
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if total != 2 || len(devices) != 2 {
		t.Errorf("devices=%+v total=%d", devices, total)
	}
}

func TestListDevicesOmitsOrgIDWhenZero(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if _, present := r.URL.Query()["orgId"]; present {
			t.Error("orgId should be omitted from the query when 0")
		}
		if _, present := r.URL.Query()["includeSubOrgDevices"]; present {
			t.Error("includeSubOrgDevices should be omitted from the query when false")
		}
		json.NewEncoder(w).Encode(map[string]any{"content": []Device{}, "totalElements": 0})
	})
	if _, _, err := client.ListDevices(0, false, 0, 50); err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
}
