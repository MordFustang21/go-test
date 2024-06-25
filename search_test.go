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
				{Name: "Test_Nested/L1/L2", File: "testdata/nested_t_run_string.go"},
			},
		},
		{
			file:  "testdata/t_run_for_loop.go",
			tests: []Test{
				{Name: "Test_ForLoop", File: "testdata/t_run_for_loop.go"},
				{Name: "Test_ForLoop/test1", File: "testdata/t_run_for_loop.go"},
				{Name: "Test_ForLoop/test2", File: "testdata/t_run_for_loop.go"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			debug = boolPtr(true)
			
			found := findTests(tt.file)
			// verify tests are found
			assert.Equal(t, tt.tests, found)
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
