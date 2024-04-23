package main

import (
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

			for _, foundTest := range found {
				found := false
				for _, expectedTest := range tt.tests {
					if foundTest == expectedTest {
						found = true
						break
					}
				}

				if !found {
					t.Fatalf("unexpcted test %s", foundTest.Name)
				}
			}
		})
	}
}
