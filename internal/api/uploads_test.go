package api

import (
	"encoding/json"
	"testing"
)

func TestUploadResponseUnmarshal(t *testing.T) {
	body := `{
		"path": "/static/fw_14668724782601366647_-838595071.yml",
		"firmwareInfo": {
			"version": "2.3.1",
			"fwType": "",
			"boardType": "Other",
			"md5Hash": "A662452975A04B0D69804AE61D8B3FF9",
			"type": "MCU",
			"fileSize": 3181
		}
	}`
	var resp UploadResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Path == "" {
		t.Error("Path is empty")
	}
	if resp.FirmwareInfo == nil || resp.FirmwareInfo.FwType != "" {
		t.Errorf("FirmwareInfo = %+v", resp.FirmwareInfo)
	}
}

func TestUploadResponseUnmarshalNullFirmwareInfo(t *testing.T) {
	var resp UploadResponse
	if err := json.Unmarshal([]byte(`{"path":"/p","firmwareInfo":null}`), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.FirmwareInfo != nil {
		t.Errorf("FirmwareInfo = %+v, want nil", resp.FirmwareInfo)
	}
}

// TestContentTypeForUpload covers the accepted-extensions list documented
// at https://docs.blynk.io/en/blynk.cloud/platform-https-api/uploads,
// including the docker-compose.yml case from live testing.
func TestContentTypeForUpload(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"firmware.bin", "application/octet-stream"},
		{"docker-compose.yml", "application/x-yaml"},
		{"manifest.YAML", "application/x-yaml"},
		{"archive.zip", "application/zip"},
		{"archive.tar", "application/x-tar"},
		{"archive.tar.gz", "application/gzip"},
		{"firmware.bin.gz", "application/gzip"},
		{"firmware.xz", "application/x-xz"},
		{"firmware.bz2", "application/x-bzip2"},
		{"unknown.ext", "application/octet-stream"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := contentTypeForUpload(tc.path); got != tc.want {
				t.Errorf("contentTypeForUpload(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
