package config

import "testing"

func TestParseConfigBytesUsesForkedManagementPanelByDefault(t *testing.T) {
	cfg, err := ParseConfigBytes([]byte("{}"))
	if err != nil {
		t.Fatalf("ParseConfigBytes() error = %v", err)
	}

	const want = "https://github.com/zcw199604/Cli-Proxy-API-Management-Center"
	if got := cfg.RemoteManagement.PanelGitHubRepository; got != want {
		t.Fatalf("panel repository = %q, want %q", got, want)
	}
}

func TestParseConfigBytesMigratesOriginalManagementPanelDefault(t *testing.T) {
	cfg, err := ParseConfigBytes([]byte(`remote-management:
  panel-github-repository: "https://github.com/router-for-me/Cli-Proxy-API-Management-Center"
`))
	if err != nil {
		t.Fatalf("ParseConfigBytes() error = %v", err)
	}

	const want = "https://github.com/zcw199604/Cli-Proxy-API-Management-Center"
	if got := cfg.RemoteManagement.PanelGitHubRepository; got != want {
		t.Fatalf("migrated panel repository = %q, want %q", got, want)
	}
}
