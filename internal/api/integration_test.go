package api

import (
	"os"
	"testing"
)

// These tests exercise real Platform API calls end-to-end against a live
// server. They're read-only (never create/mutate anything) and skipped by
// default — set BLYNK_TEST_SERVER and BLYNK_TEST_TOKEN (a static token, to
// avoid needing OAuth2 client credentials just to run tests) to opt in:
//
//	BLYNK_TEST_SERVER=fra.blynk-qa.com BLYNK_TEST_TOKEN=... go test ./internal/api/... -run Integration -v
func testClientFromEnv(t *testing.T) *Client {
	t.Helper()
	server := os.Getenv("BLYNK_TEST_SERVER")
	token := os.Getenv("BLYNK_TEST_TOKEN")
	if server == "" || token == "" {
		t.Skip("set BLYNK_TEST_SERVER and BLYNK_TEST_TOKEN to run this against a real server")
	}
	return NewClient(server, token)
}

func TestIntegrationOrganizationProfile(t *testing.T) {
	client := testClientFromEnv(t)
	org, err := client.OrganizationProfile()
	if err != nil {
		t.Fatalf("OrganizationProfile: %v", err)
	}
	if org.ID == 0 {
		t.Errorf("org.ID = 0, want a real organization id")
	}
	t.Logf("whoami: org %d (%s) on %s", org.ID, org.Name, client.Server)
}

func TestIntegrationListShipments(t *testing.T) {
	client := testClientFromEnv(t)
	shipments, err := client.ListShipments(0)
	if err != nil {
		t.Fatalf("ListShipments: %v", err)
	}
	t.Logf("found %d shipment(s) on %s", len(shipments), client.Server)
}
