package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anthony-blynk/blynk-cli/internal/api"
)

func TestUserTableIncludesCoreFields(t *testing.T) {
	d := &api.UserDetails{
		User:   api.User{ID: 213523, Name: "Anthony", Email: "anthony@blynk.cc", RoleID: 1, OrgID: 9740},
		Status: "Active",
	}
	tbl := userTable(d)
	want := map[string]string{
		"id":      "213523",
		"name":    "Anthony",
		"email":   "anthony@blynk.cc",
		"role_id": "1",
		"org_id":  "9740",
		"status":  "Active",
		"is_dev":  "false",
	}
	got := map[string]string{}
	for _, row := range tbl.Rows {
		if len(row) == 2 {
			got[row[0]] = row[1]
		}
	}
	for field, wantVal := range want {
		if got[field] != wantVal {
			t.Errorf("field %q = %q, want %q (rows: %v)", field, got[field], wantVal, tbl.Rows)
		}
	}
}

func TestUserTableOmitsBlankOptionalFields(t *testing.T) {
	d := &api.UserDetails{User: api.User{ID: 1, Name: "n"}}
	tbl := userTable(d)
	for _, row := range tbl.Rows {
		if len(row) > 0 && (row[0] == "title" || row[0] == "phone_number" || row[0] == "tz" || row[0] == "locale" || row[0] == "nick_name") {
			t.Errorf("expected %q row to be omitted when blank, got %v", row[0], tbl.Rows)
		}
	}
}

func TestUserTableIncludesOptionalFieldsWhenPresent(t *testing.T) {
	d := &api.UserDetails{
		User:        api.User{ID: 1, Name: "n"},
		Title:       "Engineer",
		PhoneNumber: "+1234567890",
		TZ:          "Europe/London",
	}
	tbl := userTable(d)
	found := map[string]bool{}
	for _, row := range tbl.Rows {
		if len(row) == 2 {
			found[row[0]] = true
		}
	}
	for _, field := range []string{"title", "phone_number", "tz"} {
		if !found[field] {
			t.Errorf("expected %q row to be present, got %v", field, tbl.Rows)
		}
	}
}

func TestFormatMillis(t *testing.T) {
	moment := time.Date(2026, 9, 12, 11, 59, 56, 0, time.UTC)
	got := formatMillis(moment.UnixMilli())
	want := moment.Format("2006-01-02 15:04:05 UTC")
	if got != want {
		t.Errorf("formatMillis = %q, want %q", got, want)
	}
}

// mockUserServer serves GET /organization/user (by id) and
// GET /organization/search/users from an in-memory user list.
func mockUserServer(t *testing.T, users []api.User) *api.Client {
	t.Helper()
	byID := map[string]api.User{}
	for _, u := range users {
		byID[strconv.FormatInt(u.ID, 10)] = u
	}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/organization/user":
			u, ok := byID[r.URL.Query().Get("userId")]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"user not found"}`))
				return
			}
			json.NewEncoder(w).Encode(api.UserDetails{User: u})
		case "/api/v1/organization/search/users":
			query := strings.ToLower(r.URL.Query().Get("query"))
			var matches []api.User
			for _, u := range users {
				if strings.Contains(strings.ToLower(u.Name), query) || strings.Contains(strings.ToLower(u.Email), query) {
					matches = append(matches, u)
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"content": matches, "totalElements": len(matches)})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := api.NewClient(srv.Listener.Addr().String(), "faketoken")
	c.HTTP = srv.Client()
	return c
}

func TestResolveUserTokenNumericID(t *testing.T) {
	client := mockUserServer(t, []api.User{{ID: 213523, Name: "Anthony", Email: "anthony@blynk.cc"}})
	u, err := resolveUserToken(client, "213523")
	if err != nil {
		t.Fatalf("resolveUserToken: %v", err)
	}
	if u.Name != "Anthony" {
		t.Errorf("Name = %q, want Anthony", u.Name)
	}
}

func TestResolveUserTokenByExactEmail(t *testing.T) {
	client := mockUserServer(t, []api.User{
		{ID: 1, Name: "Anthony", Email: "anthony@blynk.cc"},
		{ID: 2, Name: "Anthony Other", Email: "anthony2@blynk.cc"},
	})
	u, err := resolveUserToken(client, "anthony@blynk.cc")
	if err != nil {
		t.Fatalf("resolveUserToken: %v", err)
	}
	if u.ID != 1 {
		t.Errorf("expected the exact-email match (id 1), got %+v", u)
	}
}

func TestResolveUserTokenByExactName(t *testing.T) {
	client := mockUserServer(t, []api.User{
		{ID: 1, Name: "Anthony", Email: "a@x.com"},
		{ID: 2, Name: "Anthony Extra", Email: "b@x.com"},
	})
	u, err := resolveUserToken(client, "Anthony")
	if err != nil {
		t.Fatalf("resolveUserToken: %v", err)
	}
	if u.ID != 1 {
		t.Errorf("expected the exact-name match (id 1), got %+v", u)
	}
}

func TestResolveUserTokenAmbiguousIsError(t *testing.T) {
	client := mockUserServer(t, []api.User{
		{ID: 1, Name: "user-a", Email: "a@x.com"},
		{ID: 2, Name: "user-b", Email: "b@x.com"},
	})
	if _, err := resolveUserToken(client, "user"); err == nil {
		t.Fatal("expected an ambiguous-match error")
	}
}

func TestResolveUserTokenNoMatchIsError(t *testing.T) {
	client := mockUserServer(t, []api.User{{ID: 1, Name: "user-a"}})
	if _, err := resolveUserToken(client, "nonexistent"); err == nil {
		t.Fatal("expected a no-match error")
	}
}
