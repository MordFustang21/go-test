package testdata

import "testing"

func Test_ForLoop(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "test1"},
		{"test2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Log(tt.name)
		})
	}
}
