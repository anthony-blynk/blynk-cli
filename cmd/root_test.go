package cmd

import "testing"

func TestResolveIdentifier(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		flagChanged bool
		flagValue   string
		wantValue   string
		wantErr     bool
	}{
		{"positional only", []string{"cm4"}, false, "", "cm4", false},
		{"flag only", nil, true, "cm4", "cm4", false},
		{"neither is an error", nil, false, "", "", true},
		{"agreeing positional and flag is fine", []string{"cm4"}, true, "cm4", "cm4", false},
		{"conflicting positional and flag is an error", []string{"cm4"}, true, "other", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveIdentifier(tc.args, tc.flagChanged, tc.flagValue, "id")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveIdentifier() = %q, nil; want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveIdentifier(): unexpected error: %v", err)
			}
			if got != tc.wantValue {
				t.Errorf("resolveIdentifier() = %q, want %q", got, tc.wantValue)
			}
		})
	}
}

func TestResolveIdentifierInt64(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		flagChanged bool
		flagValue   int64
		wantValue   int64
		wantErr     bool
	}{
		{"positional only", []string{"31727"}, false, 0, 31727, false},
		{"flag only", nil, true, 31727, 31727, false},
		{"neither is an error", nil, false, 0, 0, true},
		{"agreeing positional and flag is fine", []string{"31727"}, true, 31727, 31727, false},
		{"conflicting positional and flag is an error", []string{"31727"}, true, 1, 0, true},
		{"non-numeric positional is an error", []string{"not-a-number"}, false, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveIdentifierInt64(tc.args, tc.flagChanged, tc.flagValue, "id")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveIdentifierInt64() = %d, nil; want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveIdentifierInt64(): unexpected error: %v", err)
			}
			if got != tc.wantValue {
				t.Errorf("resolveIdentifierInt64() = %d, want %d", got, tc.wantValue)
			}
		})
	}
}
