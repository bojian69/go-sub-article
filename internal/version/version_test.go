package version

import "testing"

func TestDefaultValues(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"Version default", Version, "dev"},
		{"BuildTime default", BuildTime, "unknown"},
		{"GitCommit default", GitCommit, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %q, want %q", tt.got, tt.expected)
			}
		})
	}
}
