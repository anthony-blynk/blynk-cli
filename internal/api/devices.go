package api

import (
	"net/url"
	"strconv"
)

// DeviceHardwareInfo is the firmware/hardware metadata reported by a
// device, returned as part of Device on both the list, search, and single
// GET endpoints.
type DeviceHardwareInfo struct {
	Version      string `json:"version"`      // firmware version
	FwType       string `json:"fwType"`       // firmware type
	BlynkVersion string `json:"blynkVersion"` // Blynk library version on the device
	BoardType    string `json:"boardType"`
	Build        string `json:"build"` // firmware build number
	// TemplateID here is the alphanumeric template id embedded in the
	// firmware itself (e.g. "TMPL0X9F") — a different value from the
	// numeric Device.TemplateID below, despite the shared field name.
	TemplateID string `json:"templateId"`
}

// LifecycleStatus is only present on the single-device GET response, not
// list/search results.
type LifecycleStatus struct {
	EventID int32  `json:"eventId"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	Icon    string `json:"icon"`
}

// Device is the device schema shared by the list, search, and single-get
// device endpoints. LifecycleStatus is only populated by GetDevice (the
// single-device endpoint) — list/search results leave it nil.
type Device struct {
	ID                 int64               `json:"id"`
	Name               string              `json:"name"`
	TemplateID         int32               `json:"templateId"`
	OriginalTemplateID int32               `json:"originalTemplateId"`
	OrgID              int64               `json:"orgId"`
	Token              string              `json:"token"` // device auth credential — never print by default
	ActivatedAt        int64               `json:"activatedAt"`
	OwnerUserID        int64               `json:"ownerUserId"`
	HardwareInfo       *DeviceHardwareInfo `json:"hardwareInfo"`
	LifecycleStatus    *LifecycleStatus    `json:"lifecycleStatus,omitempty"`
}

// GetDevice calls GET /api/v1/organization/device.
func (c *Client) GetDevice(deviceID int64) (*Device, error) {
	q := url.Values{"deviceId": {strconv.FormatInt(deviceID, 10)}}
	var d Device
	if err := c.Get("/api/v1/organization/device", q, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// devicePage is the paginated wrapper shared by the list and search
// device endpoints.
type devicePage struct {
	Content       []Device `json:"content"`
	TotalElements int32    `json:"totalElements"`
}

// SearchDevices calls GET /api/v1/organization/search/devices, letting
// `shipment deploy --device-ids` (and `device get --id`) accept a device
// name instead of requiring its numeric id up front.
func (c *Client) SearchDevices(query string) ([]Device, error) {
	q := url.Values{"query": {query}}
	var resp devicePage
	if err := c.Get("/api/v1/organization/search/devices", q, &resp); err != nil {
		return nil, err
	}
	return resp.Content, nil
}

// ListDevices calls GET /api/v1/organization/devices. size is clamped to
// [1, 1000] by the server; page is 0-indexed. Returns the page's devices
// plus the total element count across all pages.
func (c *Client) ListDevices(orgID int64, includeSubOrgDevices bool, page, size int) ([]Device, int32, error) {
	q := url.Values{
		"page": {strconv.Itoa(page)},
		"size": {strconv.Itoa(size)},
	}
	if orgID != 0 {
		q.Set("orgId", strconv.FormatInt(orgID, 10))
	}
	if includeSubOrgDevices {
		q.Set("includeSubOrgDevices", "true")
	}
	var resp devicePage
	if err := c.Get("/api/v1/organization/devices", q, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Content, resp.TotalElements, nil
}

// IsOnline calls GET /api/v1/organization/device/online, which checks
// whether a device is currently connected over MQTT/Blynk Protocol — a
// separate call from GetDevice, since connectivity isn't part of the
// device object itself.
func (c *Client) IsOnline(deviceID int64) (bool, error) {
	q := url.Values{"deviceId": {strconv.FormatInt(deviceID, 10)}}
	var resp struct {
		Connected bool `json:"connected"`
	}
	if err := c.Get("/api/v1/organization/device/online", q, &resp); err != nil {
		return false, err
	}
	return resp.Connected, nil
}
