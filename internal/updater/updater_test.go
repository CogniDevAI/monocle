package updater

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"equal versions", "0.3.0", "0.3.0", false},
		{"latest is greater patch", "0.3.0", "0.3.1", true},
		{"latest is greater minor", "0.3.0", "0.4.0", true},
		{"latest is greater major", "0.3.0", "1.0.0", true},
		{"current is greater", "0.4.0", "0.3.9", false},
		{"dev build never updates", "dev", "9.9.9", false},
		{"empty current", "", "1.0.0", false},
		{"malformed current", "abc", "1.0.0", false},
		{"malformed latest", "1.0.0", "not-a-version", false},
		{"v prefix tolerated on both", "v0.3.0", "v0.3.1", true},
		{"v prefix only on latest", "0.3.0", "v0.3.1", true},
		{"two-segment current treated as X.Y.0", "0.3", "0.3.1", true},
		{"two-segment equal", "0.3", "0.3.0", false},
		{"both two-segment, latest greater", "0.3", "0.4", true},
		{"pre-release suffix on latest", "0.3.0", "0.3.1-rc1", true},
		{"build metadata on current", "0.3.0+build.5", "0.3.1", true},
		{"only major segment is malformed", "1", "1.0.0", false},
		{"too many segments is malformed", "1.0.0.0", "1.0.1", false},
		{"negative components not allowed", "-1.0.0", "1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNewer(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestInstallURL(t *testing.T) {
	got := InstallURL()
	want := "https://raw.githubusercontent.com/CogniDevAI/monocle/main/install.sh"
	if got != want {
		t.Errorf("InstallURL() = %q, want %q", got, want)
	}
}
