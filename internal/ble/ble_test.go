package ble

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// fakeDevice answers each write with the replies registered for that
// message type, standing in for the radio.
type fakeDevice struct {
	t       *testing.T
	in      chan []byte
	replies map[string][]string
	written []map[string]any
}

func newFakeSession(t *testing.T, mtu int, replies map[string][]string) (*Session, *fakeDevice) {
	f := &fakeDevice{t: t, in: make(chan []byte, 32), replies: replies}
	s := &Session{
		write: func(b []byte) error {
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatalf("wrote invalid JSON %q: %v", b, err)
			}
			f.written = append(f.written, m)
			for _, r := range f.replies[m["t"].(string)] {
				f.in <- []byte(r)
			}
			return nil
		},
		in:  f.in,
		MTU: mtu,
	}
	return s, f
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return c
}

// Real exchange with the Linux Modbus gateway on an EG5120 (2026-11-03).
var gatewayReplies = map[string][]string{
	"info":    {`{"t": "info", "vendor": "Blynk", "tmpl_id": "DYNAMIC", "fw_type": "DYNAMIC", "fw_ver": "0.1.0", "name": "Blynk Modbus Gateway-F8XQ"}`},
	"set":     {`{"t": "set_ok"}`},
	"connect": {`{"t": "connecting"}`, `{"t": "status", "s": "connecting_cloud"}`, `{"t": "status", "s": "connected"}`},
}

func TestProvisionHappyPath(t *testing.T) {
	s, f := newFakeSession(t, 517, gatewayReplies)

	info, err := s.Info(ctx(t))
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if !info.Dynamic() || info.Name != "Blynk Modbus Gateway-F8XQ" || info.FwVer != "0.1.0" {
		t.Errorf("info = %+v", info)
	}

	cfg := Config{Token: strings.Repeat("a", 32), Host: "fra1.blynk.cloud", TemplateID: "TMPL4TuK3CZDs"}
	if err := s.Configure(ctx(t), cfg); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	set := f.written[1]
	if set["blynk"] != cfg.Token || set["host"] != "fra1.blynk.cloud" || set["templateId"] != "TMPL4TuK3CZDs" {
		t.Errorf("set = %v", set)
	}
	if _, present := set["if"]; present {
		t.Error(`"if" should be omitted without Wi-Fi credentials`)
	}

	var stages []string
	if err := s.Connect(ctx(t), func(stage string) { stages = append(stages, stage) }); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if got, want := strings.Join(stages, ","), "connecting,connecting_cloud"; got != want {
		t.Errorf("stages = %s, want %s", got, want)
	}
}

func TestConfigureOmitsEmptyTemplateAndSendsWiFi(t *testing.T) {
	s, f := newFakeSession(t, 0, gatewayReplies)
	cfg := Config{Token: strings.Repeat("a", 32), Host: "h", WiFiSSID: "net", WiFiPass: "pw"}
	if err := s.Configure(ctx(t), cfg); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	set := f.written[0]
	if _, present := set["templateId"]; present {
		t.Error("templateId should be omitted when empty (fixed firmware rejects unknown keys)")
	}
	if set["if"] != "wifi" || set["ssid"] != "net" || set["pass"] != "pw" {
		t.Errorf("set = %v", set)
	}
}

func TestConfigureRejectsBadToken(t *testing.T) {
	s, f := newFakeSession(t, 0, gatewayReplies)
	if err := s.Configure(ctx(t), Config{Token: "short"}); err == nil {
		t.Fatal("expected an error for a short token")
	}
	if len(f.written) != 0 {
		t.Error("nothing should be sent for an invalid token")
	}
}

func TestSetFailIsAnError(t *testing.T) {
	s, _ := newFakeSession(t, 0, map[string][]string{"set": {`{"t": "set_fail"}`}})
	err := s.Configure(ctx(t), Config{Token: strings.Repeat("a", 32)})
	if err == nil || !strings.Contains(err.Error(), "set_fail") {
		t.Fatalf("err = %v, want set_fail", err)
	}
}

func TestNetFailDuringConnectIsAnError(t *testing.T) {
	s, _ := newFakeSession(t, 0, map[string][]string{
		"connect": {`{"t": "connecting"}`, `{"t": "status", "s": "connecting_net"}`, `{"t": "net_fail", "reason": "sim_wrong_pin"}`},
	})
	err := s.Connect(ctx(t), nil)
	if err == nil || !strings.Contains(err.Error(), "sim_wrong_pin") {
		t.Fatalf("err = %v, want net_fail with reason", err)
	}
}

func TestSendRejectsMessageOverMTU(t *testing.T) {
	s, f := newFakeSession(t, 23, gatewayReplies) // BLE's minimum MTU: 20-byte writes
	err := s.Configure(ctx(t), Config{Token: strings.Repeat("a", 32), Host: "fra1.blynk.cloud"})
	if err == nil || !strings.Contains(err.Error(), "MTU") {
		t.Fatalf("err = %v, want an MTU error", err)
	}
	if len(f.written) != 0 {
		t.Error("an oversized message should not be written")
	}
}

func TestTimeoutWithoutReply(t *testing.T) {
	s, _ := newFakeSession(t, 0, nil)
	c, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := s.Info(c); err == nil {
		t.Fatal("expected a timeout")
	}
}

func TestIgnoresNonJSONNotifications(t *testing.T) {
	s, f := newFakeSession(t, 0, nil)
	f.in <- []byte("garbage")
	f.in <- []byte(`{"t": "info", "name": "x"}`)
	f.replies = nil
	info, err := s.Info(ctx(t))
	if err != nil || info.Name != "x" {
		t.Fatalf("info=%+v err=%v", info, err)
	}
}
