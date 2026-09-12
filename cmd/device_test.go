package cmd

import (
	"strings"
	"testing"

	"github.com/anthony-blynk/blynk-cli/internal/api"
)

func TestDeviceTableHidesTokenByDefault(t *testing.T) {
	d := &api.Device{ID: 1, Name: "boiler-3", Token: "super-secret-device-token"}
	tbl := deviceTable(d, true, false)
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
	tbl := deviceTable(d, true, true)
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
		tbl := deviceTable(&api.Device{ID: 1}, tc.online, false)
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
	tbl := deviceTable(&api.Device{ID: 1, Name: "n"}, false, false)
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
	tbl := deviceTable(d, true, false)
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
