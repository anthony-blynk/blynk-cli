package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestUserUnmarshal(t *testing.T) {
	var u User
	body := `{"id":213523,"name":"Anthony","email":"anthony@blynk.cc","roleId":1,"orgId":9740,"isDev":true}`
	if err := json.Unmarshal([]byte(body), &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if u.ID != 213523 || u.Email != "anthony@blynk.cc" || !u.IsDev {
		t.Errorf("User = %+v", u)
	}
}

func TestUserDetailsUnmarshalEmbedsUser(t *testing.T) {
	body := `{
		"id": 213523,
		"name": "Anthony",
		"email": "anthony@blynk.cc",
		"roleId": 1,
		"orgId": 9740,
		"isDev": true,
		"title": "Engineer",
		"status": "Active",
		"tz": "Europe/London",
		"lastLoggedAt": 1789200000000,
		"registeredAt": 1700000000000
	}`
	var d UserDetails
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.ID != 213523 || d.Name != "Anthony" {
		t.Errorf("embedded User fields missing: %+v", d)
	}
	if d.Status != "Active" || d.Title != "Engineer" {
		t.Errorf("UserDetails fields = %+v", d)
	}
}

func TestUserPageUnmarshal(t *testing.T) {
	var page userPage
	body := `{"content":[{"id":1,"name":"a","email":"a@x.com"}],"totalElements":1}`
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if page.TotalElements != 1 || len(page.Content) != 1 {
		t.Errorf("userPage = %+v", page)
	}
}

func TestGetUserSendsUserID(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/organization/user"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("userId"), "213523"; got != want {
			t.Errorf("userId = %q, want %q", got, want)
		}
		json.NewEncoder(w).Encode(UserDetails{User: User{ID: 213523, Name: "Anthony"}})
	})

	u, err := client.GetUser(213523)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.Name != "Anthony" {
		t.Errorf("Name = %q, want Anthony", u.Name)
	}
}

func TestListUsersSendsQueryParamsWithoutOrgID(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/organization/users"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if _, present := r.URL.Query()["orgId"]; present {
			t.Error("orgId should never be sent — this endpoint has no such parameter")
		}
		q := r.URL.Query()
		if got, want := q.Get("includeSubOrgUsers"), "true"; got != want {
			t.Errorf("includeSubOrgUsers = %q, want %q", got, want)
		}
		if got, want := q.Get("page"), "0"; got != want {
			t.Errorf("page = %q, want %q", got, want)
		}
		if got, want := q.Get("size"), "50"; got != want {
			t.Errorf("size = %q, want %q", got, want)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content":       []User{{ID: 1, Name: "a"}},
			"totalElements": 1,
		})
	})

	users, total, err := client.ListUsers(true, 0, 50)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if total != 1 || len(users) != 1 {
		t.Errorf("users=%+v total=%d", users, total)
	}
}

func TestSearchUsersSendsQuery(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/organization/search/users"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("query"), "anthony@blynk.cc"; got != want {
			t.Errorf("query = %q, want %q", got, want)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content":       []User{{ID: 1, Email: "anthony@blynk.cc"}},
			"totalElements": 1,
		})
	})

	users, err := client.SearchUsers("anthony@blynk.cc")
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(users) != 1 || users[0].Email != "anthony@blynk.cc" {
		t.Errorf("users = %+v", users)
	}
}

func TestInviteUserSendsRequestBody(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodPost; got != want {
			t.Errorf("method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/api/v1/organization/users/invite"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		var body InviteUserRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Email != "new@blynk.cc" || body.Name != "New Person" || body.RoleID != 2 {
			t.Errorf("body = %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(UserDetails{
			User:   User{ID: 999, Name: "New Person", Email: "new@blynk.cc", RoleID: 2},
			Status: "Pending",
		})
	})

	u, err := client.InviteUser(InviteUserRequest{Email: "new@blynk.cc", Name: "New Person", RoleID: 2})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	if u.ID != 999 || u.Status != "Pending" {
		t.Errorf("UserDetails = %+v", u)
	}
}

func TestInviteUserOmitsZeroOrgID(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		var m map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			t.Fatal(err)
		}
		if _, present := m["orgId"]; present {
			t.Error("orgId should be omitted from the request body when 0")
		}
		json.NewEncoder(w).Encode(UserDetails{})
	})
	if _, err := client.InviteUser(InviteUserRequest{Email: "a@b.com", Name: "n", RoleID: 1}); err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
}
