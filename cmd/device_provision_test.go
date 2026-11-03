package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/anthony-blynk/blynk-cli/internal/ble"
)

func TestDefaultProvisionName(t *testing.T) {
	cases := map[string]string{
		"Blynk Modbus Gateway-F8XQ": "Modbus Gateway-F8XQ",
		"Blynk Device-JS3A":         "Device-JS3A",
		"Custom-1234":               "Custom-1234",
		"Blynk ":                    "Blynk ",
	}
	for in, want := range cases {
		if got := defaultProvisionName(in); got != want {
			t.Errorf("defaultProvisionName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortByRSSIStrongestFirst(t *testing.T) {
	ps := []ble.Peripheral{{Name: "far", RSSI: -85}, {Name: "near", RSSI: -54}, {Name: "mid", RSSI: -70}}
	sortByRSSI(ps)
	if ps[0].Name != "near" || ps[1].Name != "mid" || ps[2].Name != "far" {
		t.Errorf("order = %+v", ps)
	}
}

func TestPickPeripheral(t *testing.T) {
	gw := ble.Peripheral{Name: "Blynk Modbus Gateway-F8XQ", Address: "2C:3B:70:E9:25:EC"}
	pi := ble.Peripheral{Name: "Blynk Device-JS3A", Address: "E4:5F:01:ED:56:E7"}

	if p, err := pickPeripheral([]ble.Peripheral{gw}, "", time.Second); err != nil || p.Address != gw.Address {
		t.Errorf("single device: p=%+v err=%v", p, err)
	}

	_, err := pickPeripheral([]ble.Peripheral{gw, pi}, "", time.Second)
	if err == nil || !strings.Contains(err.Error(), "--ble-name") || !strings.Contains(err.Error(), "JS3A") {
		t.Errorf("two devices should be an error listing both and suggesting --ble-name, got %v", err)
	}

	_, err = pickPeripheral(nil, "F8XQ", time.Second)
	if err == nil || !strings.Contains(err.Error(), "F8XQ") {
		t.Errorf("no match should mention the filter, got %v", err)
	}
}
