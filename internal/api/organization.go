package api

// Address is the nested address object on an Organization.
type Address struct {
	FullAddress string `json:"fullAddress"`
	Country     string `json:"country"`
	City        string `json:"city"`
	State       string `json:"state"`
	Zip         string `json:"zip"`
}

// Organization is the response schema for
// GET /api/v1/organization/profile and GET /api/v1/organization.
type Organization struct {
	ID             int32    `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	TZ             string   `json:"tz"`
	UnitSystem     string   `json:"unitSystem"`
	PhoneNumber    string   `json:"phoneNumber"`
	Address        *Address `json:"address"`
	LastModifiedTs int64    `json:"lastModifiedTs"`
}

// OrganizationProfile calls GET /api/v1/organization/profile, which resolves
// the organization associated with the current bearer token. It's used both
// to validate a profile at `profile add` time and to implement `auth whoami`.
func (c *Client) OrganizationProfile() (*Organization, error) {
	var org Organization
	if err := c.Get("/api/v1/organization/profile", nil, &org); err != nil {
		return nil, err
	}
	return &org, nil
}
