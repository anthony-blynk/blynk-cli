package api

import (
	"net/url"
	"strconv"
)

// User is the summary schema returned by the list/search user endpoints.
type User struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	RoleID int32  `json:"roleId"`
	OrgID  int64  `json:"orgId"`
	IsDev  bool   `json:"isDev"`
}

// UserDetails is the fuller schema returned by GET /organization/user
// (single-user get). There is no documented endpoint to resolve RoleID to
// a role name for an arbitrary user — the one endpoint that returns a role
// name (/organization/user/profile) only works with user-scoped auth, not
// the client-credentials/static-token auth this CLI uses.
type UserDetails struct {
	User
	Title          string `json:"title"`
	NickName       string `json:"nickName"`
	PhoneNumber    string `json:"phoneNumber"`
	TZ             string `json:"tz"`
	Locale         string `json:"locale"`
	Status         string `json:"status"` // Pending | Active | Inactive | Suspended
	LastModifiedTs int64  `json:"lastModifiedTs"`
	LastLoggedAt   int64  `json:"lastLoggedAt"`
	RegisteredAt   int64  `json:"registeredAt"`
}

// GetUser calls GET /api/v1/organization/user.
func (c *Client) GetUser(userID int64) (*UserDetails, error) {
	q := url.Values{"userId": {strconv.FormatInt(userID, 10)}}
	var u UserDetails
	if err := c.Get("/api/v1/organization/user", q, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// userPage is the paginated wrapper shared by the list and search user
// endpoints.
type userPage struct {
	Content       []User `json:"content"`
	TotalElements int32  `json:"totalElements"`
}

// ListUsers calls GET /api/v1/organization/users. Unlike ListDevices/
// ListTemplates, this endpoint has no orgId parameter — it's always scoped
// to the caller's own org (confirmed against the docs).
func (c *Client) ListUsers(includeSubOrgUsers bool, page, size int) ([]User, int32, error) {
	q := url.Values{
		"page": {strconv.Itoa(page)},
		"size": {strconv.Itoa(size)},
	}
	if includeSubOrgUsers {
		q.Set("includeSubOrgUsers", "true")
	}
	var resp userPage
	if err := c.Get("/api/v1/organization/users", q, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Content, resp.TotalElements, nil
}

// SearchUsers calls GET /api/v1/organization/search/users, letting
// `user get` accept a name or email instead of requiring a numeric id.
func (c *Client) SearchUsers(query string) ([]User, error) {
	q := url.Values{"query": {query}}
	var resp userPage
	if err := c.Get("/api/v1/organization/search/users", q, &resp); err != nil {
		return nil, err
	}
	return resp.Content, nil
}
