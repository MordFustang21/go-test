package main

import (
	"slices"
	"testing"
)

func Test_findTests(t *testing.T) {
	tests := []struct {
		file  string
		tests []Test
	}{
		{
			file:  "testdata/bit_of_everything.go",
			tests: []Test{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			found := findTests(tt.file)
			// verify tests are found
			if len(found) != len(tt.tests) {
				t.Fatalf("expected %d tests, got %d", len(tt.tests), len(found))
			}

			for _, test := range found {
				if !slices.Contains(tt.tests, test) {
					t.Fatalf("expected to find test %s", test.Name)
				}
			}
		})
	}
}
