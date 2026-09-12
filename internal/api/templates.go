package api

import (
	"net/url"
	"strconv"
)

// Template is a minimal projection of the template schema — just enough to
// resolve a template id to/from its name for `device list --template` and
// showing a readable template name on `device list`/`device get`. The full
// `template` command group isn't built yet; see CLAUDE.md.
type Template struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}

// GetTemplate calls GET /api/v1/organization/template.
func (c *Client) GetTemplate(templateID int32) (*Template, error) {
	q := url.Values{"templateId": {strconv.FormatInt(int64(templateID), 10)}}
	var t Template
	if err := c.Get("/api/v1/organization/template", q, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// templatePage is the paginated wrapper returned by
// GET /api/v1/organization/templates.
type templatePage struct {
	Content       []Template `json:"content"`
	TotalElements int32      `json:"totalElements"`
}

// ListTemplates calls GET /api/v1/organization/templates. There is no
// template search-by-name endpoint, so resolving a template by name means
// listing all of them and matching client-side.
func (c *Client) ListTemplates(orgID int64, page, size int) ([]Template, int32, error) {
	q := url.Values{
		"page": {strconv.Itoa(page)},
		"size": {strconv.Itoa(size)},
	}
	if orgID != 0 {
		q.Set("orgId", strconv.FormatInt(orgID, 10))
	}
	var resp templatePage
	if err := c.Get("/api/v1/organization/templates", q, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Content, resp.TotalElements, nil
}
