package api

import (
	"encoding/json"
	"testing"
)

// TestErrorBodyText covers every error shape observed or documented for the
// Platform API. The nested-object case is a real response captured live
// against a QA server (a 403 for an under-scoped OAuth client) — the
// parsing originally only handled a flat {"error":"..."} string and
// silently fell back to dumping the raw JSON blob instead of the message.
func TestErrorBodyText(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "nested error object (captured live from QA server)",
			body: `{"error":{"message":"Platform API scope is not enabled for this oauth client."}}`,
			want: "Platform API scope is not enabled for this oauth client.",
		},
		{
			name: "flat message field",
			body: `{"message":"invalid request"}`,
			want: "invalid request",
		},
		{
			name: "flat error string",
			body: `{"error":"invalid_client"}`,
			want: "invalid_client",
		},
		{
			name: "message takes precedence over error when both present",
			body: `{"message":"from message","error":{"message":"from error"}}`,
			want: "from message",
		},
		{
			name: "no recognizable shape yields empty text",
			body: `{"unrelated":"field"}`,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var eb errorBody
			if err := json.Unmarshal([]byte(tc.body), &eb); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if got := eb.text(); got != tc.want {
				t.Errorf("text() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApiErrorError(t *testing.T) {
	withMessage := &apiError{StatusCode: 403, Message: "nope", Raw: `{"error":"nope"}`}
	if got, want := withMessage.Error(), "nope (HTTP 403)"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	withoutMessage := &apiError{StatusCode: 500, Raw: "boom"}
	if got, want := withoutMessage.Error(), "HTTP 500: boom"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
