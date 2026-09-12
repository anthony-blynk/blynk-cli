package api

import (
	"encoding/json"
	"testing"
)

// TestDeviceUnmarshal uses values actually seen live in this session
// (`shipment deploy --device-ids Nvidia --dry-run` resolved device 569482,
// name "Nvidia ORIN", template 345660) as a realistic fixture.
func TestDeviceUnmarshal(t *testing.T) {
	body := `{"id":569482,"name":"Nvidia ORIN","templateId":345660,"originalTemplateId":345660,"orgId":9740}`
	var d Device
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.ID != 569482 || d.Name != "Nvidia ORIN" || d.TemplateID != 345660 {
		t.Errorf("Device = %+v", d)
	}
}

func TestDeviceSearchResponseUnmarshalPaginatedWrapper(t *testing.T) {
	body := `{
		"content": [
			{"id": 1, "name": "AntsDemoTest", "templateId": 887402}
		],
		"totalElements": 1
	}`
	var resp deviceSearchResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TotalElements != 1 {
		t.Errorf("TotalElements = %d, want 1", resp.TotalElements)
	}
	if len(resp.Content) != 1 || resp.Content[0].Name != "AntsDemoTest" {
		t.Errorf("Content = %+v", resp.Content)
	}
}
