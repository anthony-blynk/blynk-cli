package api

import (
	"net/url"
	"strconv"
)

// Device is a minimal projection of the device schema — just enough for
// `shipment deploy` to resolve a device's name and template. The full
// `device` command group (and its complete typed struct) isn't built yet;
// see CLAUDE.md.
type Device struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	TemplateID int32  `json:"templateId"`
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

// deviceSearchResponse is the paginated wrapper returned by
// GET /api/v1/organization/search/devices.
type deviceSearchResponse struct {
	Content       []Device `json:"content"`
	TotalElements int32    `json:"totalElements"`
}

// SearchDevices calls GET /api/v1/organization/search/devices, letting
// `shipment deploy --device-ids` accept a device name instead of requiring
// its numeric id up front.
func (c *Client) SearchDevices(query string) ([]Device, error) {
	q := url.Values{"query": {query}}
	var resp deviceSearchResponse
	if err := c.Get("/api/v1/organization/search/devices", q, &resp); err != nil {
		return nil, err
	}
	return resp.Content, nil
}
