//go:build darwin && !cgo

package ble

import (
	"context"
	"time"
)

// Scan is unavailable: tinygo's macOS backend (CoreBluetooth) needs cgo.
func Scan(context.Context, string, time.Duration) ([]Peripheral, error) {
	return nil, ErrUnsupported
}

// Dial is unavailable: tinygo's macOS backend (CoreBluetooth) needs cgo.
func Dial(Peripheral, time.Duration) (*Session, error) {
	return nil, ErrUnsupported
}
