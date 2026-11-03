// Package ble plays the Blynk mobile app's side of Blynk BLE provisioning
// ("Blynk.Inject"): find a device advertising the provisioning service,
// read its info, hand it an auth token (and optionally Wi-Fi credentials),
// and tell it to connect. Protocol reference:
// blynk-edge-agent/docs/blynk-ble-provisioning-protocol.md.
//
// The protocol logic here is plain Go. The radio transport (transport.go)
// uses tinygo.org/x/bluetooth, which needs cgo on macOS (CoreBluetooth), so
// a macOS build without cgo gets transport_unsupported.go instead.
package ble

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrUnsupported is returned by Scan/Dial in builds without a BLE transport.
var ErrUnsupported = errors.New("BLE provisioning isn't available in this build (macOS binaries need cgo: build on a Mac with CGO_ENABLED=1)")

// Peripheral is a device found advertising the provisioning service.
type Peripheral struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	RSSI    int16  `json:"rssi"`
	addr    any    // transport-specific address, used by Dial
}

// Info is the device's reply to {"t":"info"}.
type Info struct {
	Vendor string `json:"vendor"`
	TmplID string `json:"tmpl_id"`
	FwType string `json:"fw_type"`
	FwVer  string `json:"fw_ver"`
	Name   string `json:"name"`
}

// Dynamic reports whether the firmware is template-free: the app (and so
// this package) must then send the chosen template in the set message.
func (i *Info) Dynamic() bool { return i.FwType == "DYNAMIC" }

// Config is what gets staged on the device via {"t":"set"}.
type Config struct {
	Token      string // device auth token, always 32 chars
	Host       string // Blynk server, e.g. fra1.blynk.cloud
	TemplateID string // only for Dynamic firmware; fixed firmware rejects unknown keys
	WiFiSSID   string
	WiFiPass   string
}

type setMessage struct {
	T          string `json:"t"`
	Blynk      string `json:"blynk"`
	Host       string `json:"host,omitempty"`
	TemplateID string `json:"templateId,omitempty"`
	If         string `json:"if,omitempty"`
	SSID       string `json:"ssid,omitempty"`
	Pass       string `json:"pass,omitempty"`
}

// Session is an open provisioning link to one device.
type Session struct {
	write func([]byte) error
	in    <-chan []byte
	close func() error
	// MTU is the negotiated ATT MTU, 0 if unknown. Each message must fit in
	// one write (the protocol has no framing).
	MTU int
}

// Close disconnects from the device.
func (s *Session) Close() error {
	if s.close == nil {
		return nil
	}
	err := s.close()
	s.close = nil
	return err
}

type message struct {
	fields map[string]any
	raw    []byte
}

// kind is the message type, with status pushes flattened to their stage
// ({"t":"status","s":"connected"} -> "connected").
func (m message) kind() string {
	t, _ := m.fields["t"].(string)
	if t == "status" {
		if s, ok := m.fields["s"].(string); ok {
			return s
		}
	}
	return t
}

func (m message) failed() bool {
	k := m.kind()
	return k == "error" || strings.HasSuffix(k, "_fail")
}

func (s *Session) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if s.MTU > 0 && len(b) > s.MTU-3 {
		return fmt.Errorf("message is %d bytes, more than fits in one BLE write (MTU %d)", len(b), s.MTU)
	}
	return s.write(b)
}

func (s *Session) next(ctx context.Context) (message, error) {
	for {
		select {
		case <-ctx.Done():
			return message{}, fmt.Errorf("waiting for the device: %w", ctx.Err())
		case b, ok := <-s.in:
			if !ok {
				return message{}, errors.New("device disconnected")
			}
			var fields map[string]any
			if err := json.Unmarshal(b, &fields); err != nil {
				continue // not one of ours; the protocol is one JSON object per notification
			}
			return message{fields: fields, raw: b}, nil
		}
	}
}

// await reads messages until one of want arrives, failing on any *_fail or
// error message. Other messages are passed to seen (if non-nil).
func (s *Session) await(ctx context.Context, seen func(message), want ...string) (message, error) {
	for {
		m, err := s.next(ctx)
		if err != nil {
			return m, err
		}
		if m.failed() {
			return m, fmt.Errorf("device replied %s", m.raw)
		}
		for _, w := range want {
			if m.kind() == w {
				return m, nil
			}
		}
		if seen != nil {
			seen(m)
		}
	}
}

// Info asks the device to describe itself.
func (s *Session) Info(ctx context.Context) (*Info, error) {
	if err := s.send(map[string]string{"t": "info"}); err != nil {
		return nil, err
	}
	m, err := s.await(ctx, nil, "info")
	if err != nil {
		return nil, err
	}
	var info Info
	if err := json.Unmarshal(m.raw, &info); err != nil {
		return nil, fmt.Errorf("decode info: %w", err)
	}
	return &info, nil
}

// Configure stages cfg on the device. Nothing is applied until Connect.
func (s *Session) Configure(ctx context.Context, cfg Config) error {
	if len(cfg.Token) != 32 {
		return fmt.Errorf("auth token must be 32 characters, got %d", len(cfg.Token))
	}
	msg := setMessage{T: "set", Blynk: cfg.Token, Host: cfg.Host, TemplateID: cfg.TemplateID}
	if cfg.WiFiSSID != "" {
		msg.If, msg.SSID, msg.Pass = "wifi", cfg.WiFiSSID, cfg.WiFiPass
	}
	if err := s.send(msg); err != nil {
		return err
	}
	_, err := s.await(ctx, nil, "set_ok")
	return err
}

// Connect tells the device to apply the staged config and waits until it
// reports it's connected to Blynk. progress gets each intermediate stage
// (connecting, connecting_net, connecting_cloud).
func (s *Session) Connect(ctx context.Context, progress func(stage string)) error {
	if err := s.send(map[string]string{"t": "connect"}); err != nil {
		return err
	}
	_, err := s.await(ctx, func(m message) {
		if progress != nil {
			progress(m.kind())
		}
	}, "connected")
	return err
}
