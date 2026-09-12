package api

import (
	"encoding/json"
	"testing"
)

// TestOrganizationUnmarshalNestedAddress guards against a real bug: Address
// was originally typed as a plain string, but the live API returns a
// nested object, which failed json.Unmarshal outright the first time this
// was exercised against a real server. Schema per
// https://docs.blynk.io/en/blynk.cloud/platform-https-api/organizations.
func TestOrganizationUnmarshalNestedAddress(t *testing.T) {
	body := `{
		"id": 9740,
		"name": "Blynk",
		"description": "",
		"tz": "UTC",
		"unitSystem": "METRIC",
		"phoneNumber": "",
		"address": {
			"fullAddress": "123 Main St",
			"country": "US",
			"city": "San Mateo",
			"state": "CA",
			"zip": "94401"
		},
		"lastModifiedTs": 1789200000000
	}`

	var org Organization
	err := json.Unmarshal([]byte(body), &org)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if org.ID != 9740 {
		t.Errorf("ID = %d, want 9740", org.ID)
	}
	if org.Name != "Blynk" {
		t.Errorf("Name = %q, want Blynk", org.Name)
	}
	if org.Address == nil {
		t.Fatal("Address is nil, want a populated nested object")
	}
	if org.Address.City != "San Mateo" || org.Address.Country != "US" {
		t.Errorf("Address = %+v", org.Address)
	}
}

func TestOrganizationUnmarshalNullAddress(t *testing.T) {
	var org Organization
	err := json.Unmarshal([]byte(`{"id":1,"name":"n","address":null}`), &org)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if org.Address != nil {
		t.Errorf("Address = %+v, want nil", org.Address)
	}
}
