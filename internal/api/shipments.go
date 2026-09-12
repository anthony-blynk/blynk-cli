package api

import (
	"fmt"
	"net/url"
	"strconv"
)

// ShipmentProgress is the aggregate per-shipment device-count tally
// returned alongside a Shipment. There is no per-device status endpoint in
// the Platform API — every count field is a running total across the
// shipment's deviceIds, per
// https://docs.blynk.io/en/blynk.cloud/platform-https-api/shipments.
type ShipmentProgress struct {
	Started                  int32 `json:"started"`
	RequestSent              int32 `json:"requestSent"`
	FirmwareRequested        int32 `json:"firmwareRequested"`
	FirmwareUploaded         int32 `json:"firmwareUploaded"`
	FirmwareUploadedToMobile int32 `json:"firmwareUploadedToMobile"`
	Success                  int32 `json:"success"`
	UploadFailure            int32 `json:"uploadFailure"`
	FirmwareTypeMismatch     int32 `json:"firmwareTypeMismatch"`
	DownloadLimitReached     int32 `json:"downloadLimitReached"`
	Rollback                 int32 `json:"rollback"`
	FirmwareVersionMismatch  int32 `json:"firmwareVersionMismatch"`
	UpdatedAt                int64 `json:"updatedAt"`
}

// Failures sums every terminal-failure counter.
func (p ShipmentProgress) Failures() int32 {
	return p.UploadFailure + p.FirmwareTypeMismatch + p.DownloadLimitReached + p.Rollback + p.FirmwareVersionMismatch
}

// Shipment is the response schema for the shipment list/get/create/stop
// endpoints.
type Shipment struct {
	ID                       int64            `json:"id"`
	ProductID                int32            `json:"productId"`
	ProductName              string           `json:"productName"`
	Title                    string           `json:"title"`
	Status                   string           `json:"status"` // RUN | PAUSE | FINISH | CANCEL
	ShipmentTime             string           `json:"shipmentTime"`
	PathToFirmware           string           `json:"pathToFirmware"`
	FirmwareOriginalFileName string           `json:"firmwareOriginalFileName"`
	Host                     string           `json:"host,omitempty"`
	StartedByUserID          int64            `json:"startedByUserId"`
	StartedAt                int64            `json:"startedAt"`
	FinishedAt               int64            `json:"finishedAt"`
	DeviceIDs                []int32          `json:"deviceIds"`
	FirmwareInfo             *FirmwareInfo    `json:"firmwareInfo"`
	AttemptsLimit            int32            `json:"attemptsLimit"`
	AttemptResetPeriodMs     int64            `json:"attemptResetPeriodMs"`
	IsSecure                 bool             `json:"isSecure"`
	SkipFwTypeCheck          bool             `json:"skipFwTypeCheck"`
	IsCritical               bool             `json:"isCritical"`
	SendPush                 bool             `json:"sendPush"`
	CompareField             string           `json:"compareField"`
	ShipmentProgress         ShipmentProgress `json:"shipmentProgress"`
}

// Terminal reports whether the shipment has reached a final status.
func (s Shipment) Terminal() bool {
	return s.Status == "FINISH" || s.Status == "CANCEL"
}

// CreateShipmentRequest is the body of POST /shipment/create.
type CreateShipmentRequest struct {
	OrgID                    int64         `json:"orgId,omitempty"`
	ProductID                int32         `json:"productId"`
	Title                    string        `json:"title"`
	PathToFirmware           string        `json:"pathToFirmware"`
	FirmwareOriginalFileName string        `json:"firmwareOriginalFileName"`
	FirmwareInfo             *FirmwareInfo `json:"firmwareInfo,omitempty"`
	DeviceIDs                []int32       `json:"deviceIds"`
	ShipmentTime             string        `json:"shipmentTime,omitempty"`
	Host                     string        `json:"host,omitempty"`
	// SkipFwTypeCheck bypasses the device/template fw-type compatibility
	// check — needed for non-firmware "firmware" artifacts (e.g. a
	// docker-compose.yml shipped to a container-based device) that have no
	// meaningful fwType for the check to compare against.
	SkipFwTypeCheck bool `json:"skipFwTypeCheck,omitempty"`
	// CompareField: NO_CONDITION | BUILD_DATE_DIFFERS | EARLIER_BUILD_DATE |
	// LATEST_FIRMWARE_VERSION | LATEST_BLYNK_VERSION. Server defaults to
	// BUILD_DATE_DIFFERS when omitted.
	CompareField string `json:"compareField,omitempty"`
}

// ListShipments calls GET /api/v1/organization/shipments.
func (c *Client) ListShipments(orgID int64) ([]Shipment, error) {
	q := url.Values{}
	if orgID != 0 {
		q.Set("orgId", strconv.FormatInt(orgID, 10))
	}
	var shipments []Shipment
	if err := c.Get("/api/v1/organization/shipments", q, &shipments); err != nil {
		return nil, err
	}
	return shipments, nil
}

// GetShipment calls GET /api/v1/organization/shipment.
func (c *Client) GetShipment(shipmentID int64, orgID int64) (*Shipment, error) {
	q := url.Values{"shipmentId": {strconv.FormatInt(shipmentID, 10)}}
	if orgID != 0 {
		q.Set("orgId", strconv.FormatInt(orgID, 10))
	}
	var s Shipment
	if err := c.Get("/api/v1/organization/shipment", q, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateShipment calls POST /api/v1/organization/shipment/create.
func (c *Client) CreateShipment(req CreateShipmentRequest) (*Shipment, error) {
	var s Shipment
	if err := c.Post("/api/v1/organization/shipment/create", nil, req, &s); err != nil {
		return nil, fmt.Errorf("create shipment: %w", err)
	}
	return &s, nil
}

// StopShipment calls PUT /api/v1/organization/shipment/stop. Only shipments
// in RUN or PAUSE status can be stopped.
func (c *Client) StopShipment(shipmentID int64, orgID int64) (*Shipment, error) {
	q := url.Values{"shipmentId": {strconv.FormatInt(shipmentID, 10)}}
	if orgID != 0 {
		q.Set("orgId", strconv.FormatInt(orgID, 10))
	}
	var s Shipment
	if err := c.Put("/api/v1/organization/shipment/stop", q, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteShipment calls DELETE /api/v1/organization/shipment. Only shipments
// in FINISH or CANCEL status can be deleted.
func (c *Client) DeleteShipment(shipmentID int64, orgID int64) error {
	q := url.Values{"shipmentId": {strconv.FormatInt(shipmentID, 10)}}
	if orgID != 0 {
		q.Set("orgId", strconv.FormatInt(orgID, 10))
	}
	return c.Delete("/api/v1/organization/shipment", q, nil)
}
