package main

import "testing"

func Test_packageFromPathAndMod(t *testing.T) {
	tests := []struct {
		name string
		path string
		mod  string
		out  string
	}{
		{
			name: "Basic",
			path: "/Users/user/main.go",
			mod:  "/Users/user/",
			out:  "main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := packageFromPathAndMod(tt.path, tt.mod); got != tt.out {
				t.Errorf("packageFromPathAndMod() = %v, want %v", got, tt.out)
			}
		})
	}
}
