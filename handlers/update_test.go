package handlers

import (
	"encoding/json"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in   string
		want [3]int
	}{
		{"v1.2.3", [3]int{1, 2, 3}},
		{"1.2.3", [3]int{1, 2, 3}},
		{"v10.0.0", [3]int{10, 0, 0}},
		{"v1.2", [3]int{1, 2, 0}},
		{"v1.2.3-rc1", [3]int{1, 2, 3}},
		{"dev", [3]int{0, 0, 0}},
		{"", [3]int{0, 0, 0}},
	}

	for _, tt := range tests {
		if got := parseVersion(tt.in); got != tt.want {
			t.Fatalf("parseVersion(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestIsVersionNewer(t *testing.T) {
	if !isVersionNewer("v1.2.4", "v1.2.3") {
		t.Fatalf("expected newer version")
	}
	if isVersionNewer("v1.2.3", "v1.2.3") {
		t.Fatalf("expected not newer for equal versions")
	}
	if isVersionNewer("v1.2.3", "v1.2.4") {
		t.Fatalf("expected not newer for older versions")
	}
}

func TestSelectReleaseAsset(t *testing.T) {
	raw := `{
  "tag_name": "v1.10.0",
  "html_url": "https://example.invalid/release",
  "assets": [
    { "name": "bastion-v1.10.0-linux-amd64.tar.gz", "browser_download_url": "https://example.invalid/linux-amd64" },
    { "name": "bastion-v1.10.0-linux-arm64.tar.gz", "browser_download_url": "https://example.invalid/linux-arm64" },
    { "name": "bastion-v1.10.0-windows-amd64.zip", "browser_download_url": "https://example.invalid/windows-amd64" }
  ]
}`

	var release githubRelease
	if err := json.Unmarshal([]byte(raw), &release); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	name, url, err := selectReleaseAsset(&release, "linux", "amd64")
	if err != nil {
		t.Fatalf("selectReleaseAsset: %v", err)
	}
	if name != "bastion-v1.10.0-linux-amd64.tar.gz" || url == "" {
		t.Fatalf("unexpected selection: name=%q url=%q", name, url)
	}

	name, _, err = selectReleaseAsset(&release, "windows", "amd64")
	if err != nil {
		t.Fatalf("selectReleaseAsset: %v", err)
	}
	if name != "bastion-v1.10.0-windows-amd64.zip" {
		t.Fatalf("unexpected selection for windows: %q", name)
	}

	if _, _, err := selectReleaseAsset(&release, "darwin", "amd64"); err == nil {
		t.Fatalf("expected error for missing darwin asset")
	}
}
