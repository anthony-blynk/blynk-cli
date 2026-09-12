package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// FirmwareInfo is metadata parsed server-side from an uploaded firmware
// file; nil/zero when parsing fails.
type FirmwareInfo struct {
	Version      string `json:"version"`
	BlynkVersion string `json:"blynkVersion"`
	FwType       string `json:"fwType"`
	BoardType    string `json:"boardType"`
	BuildDate    string `json:"buildDate"`
	MD5Hash      string `json:"md5Hash"`
	SHA256Hash   string `json:"sha256Hash"`
	Type         string `json:"type"` // NCP | MCU
	FileSize     int32  `json:"fileSize"`
}

// UploadResponse is the body of POST /api/upload.
type UploadResponse struct {
	Path         string        `json:"path"`
	FirmwareInfo *FirmwareInfo `json:"firmwareInfo"`
}

// UploadFirmware uploads a firmware file via POST /api/upload
// (multipart/form-data, field "upfile" + "type"="FIRMWARE"), per
// https://docs.blynk.io/en/blynk.cloud/platform-https-api/uploads.
// The returned path/firmwareInfo feed pathToFirmware/firmwareInfo on
// shipment create.
func (c *Client) UploadFirmware(filePath string) (*UploadResponse, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	if err := w.WriteField("type", "FIRMWARE"); err != nil {
		return nil, fmt.Errorf("write type field: %w", err)
	}
	part, err := w.CreateFormFile("upfile", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	url := fmt.Sprintf("https://%s/api/upload", c.Server)
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("build upload request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload %s: %w", filePath, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upload response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &apiError{StatusCode: resp.StatusCode, Raw: string(respBody)}
		var eb errorBody
		if json.Unmarshal(respBody, &eb) == nil {
			if eb.Message != "" {
				apiErr.Message = eb.Message
			} else if eb.Error != "" {
				apiErr.Message = eb.Error
			}
		}
		return nil, apiErr
	}

	var out UploadResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("parse upload response: %w", err)
	}
	return &out, nil
}
