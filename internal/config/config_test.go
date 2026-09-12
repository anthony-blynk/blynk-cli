package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setHome points os.UserHomeDir() (and thus Dir()/Path()) at dir for the
// duration of the test.
func setHome(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	setHome(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load (no file yet): %v", err)
	}
	if len(cfg.Profiles) != 0 {
		t.Fatalf("expected empty profiles, got %v", cfg.Profiles)
	}

	cfg.Set("acme-prod", Profile{Server: "acme.blynk.cloud", ClientID: "abc123", ClientSecret: "secret"})
	cfg.Set("acme-qa", Profile{Server: "acme-qa.blynk-qa.com", Token: "statictoken"})
	cfg.CurrentProfile = "acme-prod"

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load (after save): %v", err)
	}
	if reloaded.CurrentProfile != "acme-prod" {
		t.Errorf("CurrentProfile = %q, want acme-prod", reloaded.CurrentProfile)
	}
	if len(reloaded.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(reloaded.Profiles))
	}
	prod, ok := reloaded.Get("acme-prod")
	if !ok || prod.Server != "acme.blynk.cloud" || prod.ClientID != "abc123" || prod.ClientSecret != "secret" {
		t.Errorf("acme-prod profile = %+v", prod)
	}
	qa, ok := reloaded.Get("acme-qa")
	if !ok || qa.Token != "statictoken" || !qa.IsStaticToken() {
		t.Errorf("acme-qa profile = %+v", qa)
	}
}

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	setHome(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Profiles == nil {
		t.Error("Profiles map should be initialized, not nil")
	}
	if cfg.CurrentProfile != "" {
		t.Errorf("CurrentProfile = %q, want empty", cfg.CurrentProfile)
	}
}

func TestLoadStripsUTF8BOM(t *testing.T) {
	dir := t.TempDir()
	setHome(t, dir)

	blynkDir := filepath.Join(dir, ".blynk")
	if err := os.MkdirAll(blynkDir, 0o700); err != nil {
		t.Fatal(err)
	}
	bom := []byte{0xEF, 0xBB, 0xBF}
	content := append(bom, []byte("current_profile: acme-prod\nprofiles:\n  acme-prod:\n    server: acme.blynk.cloud\n    token: t\n")...)
	if err := os.WriteFile(filepath.Join(blynkDir, "config.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CurrentProfile != "acme-prod" {
		t.Errorf("CurrentProfile = %q, want acme-prod (BOM not stripped?)", cfg.CurrentProfile)
	}
	if p, ok := cfg.Get("acme-prod"); !ok || p.Server != "acme.blynk.cloud" {
		t.Errorf("acme-prod profile = %+v, ok=%v", p, ok)
	}
}

func TestRemoveClearsCurrentProfile(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"a": {Server: "s"}}, CurrentProfile: "a"}
	cfg.Remove("a")
	if cfg.CurrentProfile != "" {
		t.Errorf("CurrentProfile = %q, want empty after removing the current profile", cfg.CurrentProfile)
	}
	if _, ok := cfg.Get("a"); ok {
		t.Error("profile 'a' should be gone")
	}
}

func TestRemoveOtherProfileKeepsCurrentProfile(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"a": {}, "b": {}}, CurrentProfile: "a"}
	cfg.Remove("b")
	if cfg.CurrentProfile != "a" {
		t.Errorf("CurrentProfile = %q, want a (removing an unrelated profile shouldn't clear it)", cfg.CurrentProfile)
	}
}

func TestNamesSorted(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"zebra": {}, "acme": {}, "middle": {}}}
	got := cfg.Names()
	want := []string{"acme", "middle", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Names() = %v, want %v", got, want)
		}
	}
}

func TestResolveExactMatch(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"acme-prod": {}, "acme-qa": {}}}
	got, err := cfg.Resolve("acme-prod")
	if err != nil || got != "acme-prod" {
		t.Fatalf("Resolve(exact) = %q, %v", got, err)
	}
}

func TestResolveUnambiguousPrefix(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"acme-prod": {}, "streetleaf-prod": {}}}
	got, err := cfg.Resolve("acme")
	if err != nil || got != "acme-prod" {
		t.Fatalf("Resolve(prefix) = %q, %v", got, err)
	}
}

func TestResolveAmbiguousPrefixIsError(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"acme-prod": {}, "acme-qa": {}}}
	_, err := cfg.Resolve("acme")
	if err == nil {
		t.Fatal("expected an ambiguous-match error, got nil")
	}
}

func TestResolveSubstringFallback(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"customer-acme-prod": {}}}
	got, err := cfg.Resolve("acme")
	if err != nil || got != "customer-acme-prod" {
		t.Fatalf("Resolve(substring) = %q, %v", got, err)
	}
}

func TestResolveNoMatchIsError(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"acme-prod": {}}}
	if _, err := cfg.Resolve("nonexistent"); err == nil {
		t.Fatal("expected a no-match error, got nil")
	}
}

func TestIsStaticToken(t *testing.T) {
	cases := []struct {
		name string
		p    Profile
		want bool
	}{
		{"static token profile", Profile{Server: "s", Token: "t"}, true},
		{"client-credentials profile", Profile{Server: "s", ClientID: "id", ClientSecret: "secret"}, false},
		{"empty profile", Profile{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.IsStaticToken(); got != tc.want {
				t.Errorf("IsStaticToken() = %v, want %v", got, tc.want)
			}
		})
	}
}
