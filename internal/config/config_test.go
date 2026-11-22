package config

import "testing"

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"0": 0, "1048576": 1 << 20, "200GiB": 200 << 30, "50GB": 50e9, "1 TiB": 1 << 40, "10B": 10} {
		got, err := ParseSize(in)
		if err != nil || got != want {
			t.Errorf("ParseSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "GiB", "-1", "1.5GB", "ten"} {
		if _, err := ParseSize(in); err == nil {
			t.Errorf("ParseSize(%q) did not fail", in)
		}
	}
}
