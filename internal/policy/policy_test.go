package policy

import (
	"testing"
	"time"
)

func TestEvaluate(t *testing.T) {
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	cd := 14 * 24 * time.Hour
	old := now.Add(-30 * 24 * time.Hour)
	recent := now.Add(-2 * 24 * time.Hour)

	tests := []struct {
		name  string
		rules []Rule
		file  File
		want  Status
	}{
		{"old file", nil, File{Version: "1.0", UploadTime: &old}, Visible},
		{"new file", nil, File{Version: "1.0", UploadTime: &recent}, Cooldown},
		{"no upload time, first seen recently", nil, File{Version: "1.0", FirstSeen: recent}, Cooldown},
		{"no upload time, first seen long ago", nil, File{Version: "1.0", FirstSeen: old}, Visible},
		{"upload time wins over first seen", nil, File{Version: "1.0", UploadTime: &old, FirstSeen: recent}, Visible},
		{"override one version", []Rule{{Kind: "cooldown_override", Version: "1.0"}}, File{Version: "1.0", UploadTime: &recent}, Override},
		{"override other version", []Rule{{Kind: "cooldown_override", Version: "2.0"}}, File{Version: "1.0", UploadTime: &recent}, Cooldown},
		{"allow bypasses cooldown", []Rule{{Kind: "allow"}}, File{Version: "1.0", UploadTime: &recent}, Allowed},
		{"block all versions", []Rule{{Kind: "block"}}, File{Version: "1.0", UploadTime: &old}, Blocked},
		{"block wins over allow", []Rule{{Kind: "allow"}, {Kind: "block", Version: "1.0"}}, File{Version: "1.0", UploadTime: &old}, Blocked},
		{"removed upstream wins", []Rule{{Kind: "allow"}}, File{Version: "1.0", UploadTime: &old, RemovedUpstream: true}, Removed},
		{"version match ignores case", []Rule{{Kind: "block", Version: "1.0RC1"}}, File{Version: "1.0rc1", UploadTime: &old}, Blocked},
	}
	for _, tt := range tests {
		if got := Evaluate(tt.rules, tt.file, cd, now); got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, got, tt.want)
		}
	}
	if Cooldown.Visible() || Blocked.Visible() || Removed.Visible() || !Override.Visible() {
		t.Error("Visible() is wrong")
	}
}
