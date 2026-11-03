//go:build !darwin || cgo

package ble

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	serviceUUID = mustUUID("95e30001-5737-45a9-a092-a88e2e5dd659")
	rxUUID      = mustUUID("95e30002-5737-45a9-a092-a88e2e5dd659") // app -> device writes
	txUUID      = mustUUID("95e30003-5737-45a9-a092-a88e2e5dd659") // device -> app notifications
)

func mustUUID(s string) bluetooth.UUID {
	u, err := bluetooth.ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return u
}

var (
	enableOnce sync.Once
	enableErr  error
)

func adapter() (*bluetooth.Adapter, error) {
	a := bluetooth.DefaultAdapter
	enableOnce.Do(func() { enableErr = a.Enable() })
	if enableErr != nil {
		return nil, fmt.Errorf("enable Bluetooth adapter: %w", enableErr)
	}
	return a, nil
}

// Scan looks for devices advertising the provisioning service whose name
// contains nameFilter (case-insensitive; "" matches any). It returns once
// linger has passed since the first match — so a second device advertising
// at the same time still shows up — or when ctx ends, whichever is first.
func Scan(ctx context.Context, nameFilter string, linger time.Duration) ([]Peripheral, error) {
	a, err := adapter()
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	found := map[string]Peripheral{}
	var order []string
	firstMatch := make(chan struct{})
	filter := strings.ToLower(nameFilter)

	done := make(chan error, 1)
	go func() {
		done <- a.Scan(func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
			name := r.LocalName()
			if name == "" || !r.HasServiceUUID(serviceUUID) || !strings.Contains(strings.ToLower(name), filter) {
				return
			}
			addr := r.Address.String()
			mu.Lock()
			defer mu.Unlock()
			if _, seen := found[addr]; !seen {
				order = append(order, addr)
				if len(order) == 1 {
					close(firstMatch)
				}
			}
			found[addr] = Peripheral{Name: name, Address: addr, RSSI: r.RSSI, addr: r.Address}
		})
	}()

	select {
	case <-firstMatch:
		select {
		case <-time.After(linger):
		case <-ctx.Done():
		}
	case <-ctx.Done():
	case err := <-done:
		return nil, fmt.Errorf("scan: %w", err)
	}
	a.StopScan()
	<-done

	mu.Lock()
	defer mu.Unlock()
	out := make([]Peripheral, len(order))
	for i, addr := range order {
		out[i] = found[addr]
	}
	return out, nil
}

// Dial connects to p and subscribes to its notifications. Windows brings
// the link up lazily and took 11-16 s in testing, so connectTimeout should
// be well above tinygo's 10 s default.
func Dial(p Peripheral, connectTimeout time.Duration) (*Session, error) {
	a, err := adapter()
	if err != nil {
		return nil, err
	}
	addr, ok := p.addr.(bluetooth.Address)
	if !ok {
		return nil, fmt.Errorf("peripheral %q wasn't found by Scan", p.Name)
	}

	// Connecting right as the scan stops failed in testing alongside the
	// short timeout; a brief pause is cheap insurance.
	time.Sleep(500 * time.Millisecond)
	dev, err := a.Connect(addr, bluetooth.ConnectionParams{ConnectionTimeout: bluetooth.NewDuration(connectTimeout)})
	if err != nil {
		return nil, fmt.Errorf("connect to %q: %w", p.Name, err)
	}
	fail := func(format string, args ...any) (*Session, error) {
		dev.Disconnect()
		return nil, fmt.Errorf(format, args...)
	}

	svcs, err := dev.DiscoverServices([]bluetooth.UUID{serviceUUID})
	if err != nil || len(svcs) == 0 {
		return fail("discover provisioning service on %q: %v", p.Name, err)
	}
	chars, err := svcs[0].DiscoverCharacteristics([]bluetooth.UUID{rxUUID, txUUID})
	if err != nil {
		return fail("discover characteristics on %q: %w", p.Name, err)
	}
	var rx, tx *bluetooth.DeviceCharacteristic
	for i := range chars {
		switch chars[i].UUID() {
		case rxUUID:
			rx = &chars[i]
		case txUUID:
			tx = &chars[i]
		}
	}
	if rx == nil || tx == nil {
		return fail("%q is missing the provisioning RX/TX characteristics", p.Name)
	}

	in := make(chan []byte, 32)
	err = tx.EnableNotifications(func(buf []byte) {
		msg := append([]byte(nil), buf...) // the backend may reuse buf
		select {
		case in <- msg:
		default: // nobody's reading fast enough; drop rather than block the BLE stack
		}
	})
	if err != nil {
		return fail("subscribe to %q: %w", p.Name, err)
	}

	s := &Session{
		write: func(b []byte) error {
			_, err := rx.WriteWithoutResponse(b)
			return err
		},
		in:    in,
		close: dev.Disconnect,
	}
	if mtu, err := rx.GetMTU(); err == nil {
		s.MTU = int(mtu)
	}
	return s, nil
}
