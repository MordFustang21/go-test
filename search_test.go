package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_findTests(t *testing.T) {
	tests := []struct {
		file  string
		tests []Test
	}{
		{
			file: "testdata/nested_t_run_string.go",
			tests: []Test{
				{Name: "Test_Nested", File: "testdata/nested_t_run_string.go"},
				{Name: "Test_Nested/L1", File: "testdata/nested_t_run_string.go"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			found := findTests(tt.file)
			// verify tests are found
			assert.Equal(t, tt.tests, found)
		})
	}
}
