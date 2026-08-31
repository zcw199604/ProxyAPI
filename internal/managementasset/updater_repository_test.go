package managementasset

import "testing"

func TestResolveReleaseURLUsesForkedManagementPanelByDefault(t *testing.T) {
	const want = "https://api.github.com/repos/zcw199604/Cli-Proxy-API-Management-Center/releases/latest"
	if got := resolveReleaseURL(""); got != want {
		t.Fatalf("default release URL = %q, want %q", got, want)
	}
}

func TestFallbackManagementAssetUsesForkedManagementPanel(t *testing.T) {
	const want = "https://github.com/zcw199604/Cli-Proxy-API-Management-Center/releases/latest/download/management.html"
	if defaultManagementFallbackURL != want {
		t.Fatalf("fallback management URL = %q, want %q", defaultManagementFallbackURL, want)
	}
}
