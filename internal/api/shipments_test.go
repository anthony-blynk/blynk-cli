package api

import (
	"encoding/json"
	"testing"
)

// realShipmentGetResponse is captured verbatim from a live QA server run
// (`shipment get --id 31727 -o json`) during the docker-compose.yml /
// Nvidia ORIN firmware-type-mismatch investigation. Real fixture rather
// than a synthetic one, so it locks in the exact shape the server sends —
// including fwType coming back as an empty string, which is what led to
// the auto-skip-fw-type-check behavior in cmd/shipment.go.
const realShipmentGetResponse = `{
  "id": 31727,
  "productId": 345660,
  "productName": "Linux Agent",
  "title": "Nvidia ORIN · docker-compose.yml · 2026-09-12 11:19",
  "status": "FINISH",
  "shipmentTime": "ANY",
  "pathToFirmware": "/static/fw_14668724782601366647_-838595071.yml",
  "firmwareOriginalFileName": "docker-compose.yml",
  "startedByUserId": 213523,
  "startedAt": 1789208357908,
  "finishedAt": 1789208397034,
  "deviceIds": [569482],
  "firmwareInfo": {
    "version": "2.3.1",
    "blynkVersion": "",
    "fwType": "",
    "boardType": "Other",
    "buildDate": "",
    "md5Hash": "A662452975A04B0D69804AE61D8B3FF9",
    "sha256Hash": "b+f5hzru6CIfUeZ8O3YHONvKwiTpasAV80T4P/a52Qg=",
    "type": "MCU",
    "fileSize": 3181
  },
  "attemptsLimit": 0,
  "attemptResetPeriodMs": 86400000,
  "isSecure": false,
  "skipFwTypeCheck": false,
  "isCritical": false,
  "sendPush": false,
  "compareField": "BUILD_DATE_DIFFERS",
  "shipmentProgress": {
    "started": 0,
    "requestSent": 0,
    "firmwareRequested": 0,
    "firmwareUploaded": 0,
    "firmwareUploadedToMobile": 0,
    "success": 0,
    "uploadFailure": 0,
    "firmwareTypeMismatch": 1,
    "downloadLimitReached": 0,
    "rollback": 0,
    "firmwareVersionMismatch": 0,
    "updatedAt": 1789208397037
  }
}`

func TestShipmentUnmarshalRealFixture(t *testing.T) {
	var s Shipment
	if err := json.Unmarshal([]byte(realShipmentGetResponse), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s.ID != 31727 {
		t.Errorf("ID = %d, want 31727", s.ID)
	}
	if s.ProductID != 345660 || s.ProductName != "Linux Agent" {
		t.Errorf("ProductID/ProductName = %d/%q", s.ProductID, s.ProductName)
	}
	if s.Status != "FINISH" {
		t.Errorf("Status = %q, want FINISH", s.Status)
	}
	if !s.Terminal() {
		t.Error("Terminal() = false for a FINISH shipment, want true")
	}
	if len(s.DeviceIDs) != 1 || s.DeviceIDs[0] != 569482 {
		t.Errorf("DeviceIDs = %v", s.DeviceIDs)
	}

	if s.FirmwareInfo == nil {
		t.Fatal("FirmwareInfo is nil")
	}
	if s.FirmwareInfo.FwType != "" {
		t.Errorf("FwType = %q, want empty (this is the real value that drove the auto-skip fix)", s.FirmwareInfo.FwType)
	}
	if s.FirmwareInfo.Type != "MCU" {
		t.Errorf("Type = %q, want MCU (distinct field from FwType — the UI's confusing 'Firmware Type' label)", s.FirmwareInfo.Type)
	}
	if s.FirmwareInfo.Version != "2.3.1" {
		t.Errorf("Version = %q, want 2.3.1", s.FirmwareInfo.Version)
	}

	p := s.ShipmentProgress
	if p.FirmwareTypeMismatch != 1 {
		t.Errorf("FirmwareTypeMismatch = %d, want 1", p.FirmwareTypeMismatch)
	}
	if got := p.Failures(); got != 1 {
		t.Errorf("Failures() = %d, want 1", got)
	}
	if p.Success != 0 {
		t.Errorf("Success = %d, want 0", p.Success)
	}
}

func TestShipmentTerminal(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"RUN", false},
		{"PAUSE", false},
		{"FINISH", true},
		{"CANCEL", true},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			s := Shipment{Status: tc.status}
			if got := s.Terminal(); got != tc.want {
				t.Errorf("Terminal() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShipmentProgressFailures(t *testing.T) {
	p := ShipmentProgress{
		UploadFailure:           1,
		FirmwareTypeMismatch:    2,
		DownloadLimitReached:    3,
		Rollback:                4,
		FirmwareVersionMismatch: 5,
		Success:                 100, // shouldn't count toward failures
	}
	if got, want := p.Failures(), int32(1+2+3+4+5); got != want {
		t.Errorf("Failures() = %d, want %d", got, want)
	}
}

// TestCreateShipmentRequestOmitsZeroOptionals guards the JSON shape sent to
// POST /shipment/create: zero-value optional fields should be omitted so a
// caller that doesn't set them gets the server's own defaults rather than
// explicit false/0/"" overriding something meaningful.
func TestCreateShipmentRequestOmitsZeroOptionals(t *testing.T) {
	req := CreateShipmentRequest{
		ProductID:                1,
		Title:                    "t",
		PathToFirmware:           "/p",
		FirmwareOriginalFileName: "f.bin",
		DeviceIDs:                []int32{1},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"orgId", "shipmentTime", "host", "skipFwTypeCheck", "compareField", "attemptsLimit", "attemptResetPeriodMs", "firmwareInfo"} {
		if _, present := m[field]; present {
			t.Errorf("field %q present in JSON with zero value, want omitted: %s", field, data)
		}
	}
}

func TestCreateShipmentRequestIncludesExplicitSkipFwTypeCheck(t *testing.T) {
	req := CreateShipmentRequest{SkipFwTypeCheck: true}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if string(m["skipFwTypeCheck"]) != "true" {
		t.Errorf("skipFwTypeCheck = %s, want true", m["skipFwTypeCheck"])
	}
}
