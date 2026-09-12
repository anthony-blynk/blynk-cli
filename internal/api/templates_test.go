package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTemplateUnmarshal(t *testing.T) {
	var tpl Template
	if err := json.Unmarshal([]byte(`{"id":345660,"name":"Linux Agent","orgId":9740}`), &tpl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tpl.ID != 345660 || tpl.Name != "Linux Agent" {
		t.Errorf("Template = %+v", tpl)
	}
}

func TestTemplatePageUnmarshal(t *testing.T) {
	var page templatePage
	body := `{"content":[{"id":1,"name":"a"},{"id":2,"name":"b"}],"totalElements":2}`
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if page.TotalElements != 2 || len(page.Content) != 2 {
		t.Errorf("templatePage = %+v", page)
	}
}

func TestGetTemplateSendsTemplateID(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/organization/template"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("templateId"), "345660"; got != want {
			t.Errorf("templateId = %q, want %q", got, want)
		}
		json.NewEncoder(w).Encode(Template{ID: 345660, Name: "Linux Agent"})
	})

	tpl, err := client.GetTemplate(345660)
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if tpl.Name != "Linux Agent" {
		t.Errorf("Name = %q, want Linux Agent", tpl.Name)
	}
}

func TestListTemplatesSendsQueryParams(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/organization/templates"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		q := r.URL.Query()
		if got, want := q.Get("orgId"), "9740"; got != want {
			t.Errorf("orgId = %q, want %q", got, want)
		}
		if got, want := q.Get("page"), "0"; got != want {
			t.Errorf("page = %q, want %q", got, want)
		}
		if got, want := q.Get("size"), "200"; got != want {
			t.Errorf("size = %q, want %q", got, want)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content":       []Template{{ID: 1, Name: "a"}},
			"totalElements": 1,
		})
	})

	templates, total, err := client.ListTemplates(9740, 0, 200)
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if total != 1 || len(templates) != 1 {
		t.Errorf("templates=%+v total=%d", templates, total)
	}
}

func TestListTemplatesOmitsOrgIDWhenZero(t *testing.T) {
	client := mockAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if _, present := r.URL.Query()["orgId"]; present {
			t.Error("orgId should be omitted from the query when 0")
		}
		json.NewEncoder(w).Encode(map[string]any{"content": []Template{}, "totalElements": 0})
	})
	if _, _, err := client.ListTemplates(0, 0, 50); err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
}
