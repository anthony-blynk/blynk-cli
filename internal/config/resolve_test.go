package config

import (
	"os"
	"path/filepath"
	"testing"
)

func testConfig() *Config {
	return &Config{
		Profiles: map[string]Profile{
			"acme-prod": {Server: "acme.blynk.cloud", ClientID: "abc", ClientSecret: "s"},
			"acme-qa":   {Server: "acme-qa.blynk-qa.com", Token: "t"},
		},
		CurrentProfile: "acme-prod",
	}
}

func TestResolvePrecedence_ProfileFlagWinsOverEverything(t *testing.T) {
	cfg := testConfig()
	t.Setenv("BLYNK_PROFILE", "acme-prod") // would resolve differently if honored

	got, err := Resolve(cfg, ResolveOptions{ProfileFlag: "acme-qa"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "acme-qa" {
		t.Errorf("ProfileName = %q, want acme-qa", got.ProfileName)
	}
}

func TestResolvePrecedence_FullFlagsBypassProfiles(t *testing.T) {
	cfg := testConfig()
	t.Setenv("BLYNK_PROFILE", "acme-qa")

	got, err := Resolve(cfg, ResolveOptions{Server: "ci.blynk.cloud", Token: "citoken"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "" {
		t.Errorf("ProfileName = %q, want empty (flags should bypass profiles)", got.ProfileName)
	}
	if got.Profile.Server != "ci.blynk.cloud" || got.Profile.Token != "citoken" {
		t.Errorf("Profile = %+v", got.Profile)
	}
}

func TestResolvePrecedence_ClientCredentialFlagsBypassProfiles(t *testing.T) {
	cfg := testConfig()
	got, err := Resolve(cfg, ResolveOptions{Server: "ci.blynk.cloud", ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "" || got.Profile.ClientID != "id" {
		t.Errorf("got %+v", got)
	}
}

func TestResolvePrecedence_PartialFlagsDoNotBypass(t *testing.T) {
	// --server alone (no token, no client-id+secret) isn't "fully specified",
	// so it should fall through to lower-precedence sources instead of
	// erroring or bypassing profiles with an incomplete credential set.
	cfg := testConfig()
	got, err := Resolve(cfg, ResolveOptions{Server: "ci.blynk.cloud"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "acme-prod" {
		t.Errorf("ProfileName = %q, want acme-prod (current_profile fallback)", got.ProfileName)
	}
}

func TestResolvePrecedence_EnvVarWinsOverDotFileAndCurrentProfile(t *testing.T) {
	cfg := testConfig()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".blynk-profile"), []byte("acme-prod"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLYNK_PROFILE", "acme-qa")

	got, err := Resolve(cfg, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "acme-qa" {
		t.Errorf("ProfileName = %q, want acme-qa ($BLYNK_PROFILE should win over .blynk-profile)", got.ProfileName)
	}
}

func TestResolvePrecedence_DotFileWinsOverCurrentProfile(t *testing.T) {
	cfg := testConfig() // CurrentProfile is acme-prod
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".blynk-profile"), []byte("acme-qa\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(cfg, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "acme-qa" {
		t.Errorf("ProfileName = %q, want acme-qa (.blynk-profile should win over current_profile)", got.ProfileName)
	}
}

func TestResolvePrecedence_DotFileFoundInParentDir(t *testing.T) {
	cfg := testConfig()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".blynk-profile"), []byte("acme-qa"), 0o600); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)

	got, err := Resolve(cfg, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "acme-qa" {
		t.Errorf("ProfileName = %q, want acme-qa (should walk up to find .blynk-profile)", got.ProfileName)
	}
}

func TestResolvePrecedence_DotFileStripsUTF8BOM(t *testing.T) {
	cfg := testConfig()
	dir := t.TempDir()
	t.Chdir(dir)
	bom := []byte{0xEF, 0xBB, 0xBF}
	if err := os.WriteFile(filepath.Join(dir, ".blynk-profile"), append(bom, []byte("acme-qa")...), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(cfg, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "acme-qa" {
		t.Errorf("ProfileName = %q, want acme-qa (BOM should be stripped)", got.ProfileName)
	}
}

func TestResolvePrecedence_CurrentProfileFallback(t *testing.T) {
	cfg := testConfig()
	t.Chdir(t.TempDir()) // no .blynk-profile anywhere up this tree

	got, err := Resolve(cfg, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "acme-prod" {
		t.Errorf("ProfileName = %q, want acme-prod", got.ProfileName)
	}
}

func TestResolvePrecedence_NoProfileSelectedIsError(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	t.Chdir(t.TempDir())

	if _, err := Resolve(cfg, ResolveOptions{}); err == nil {
		t.Fatal("expected an error when nothing selects a profile")
	}
}
