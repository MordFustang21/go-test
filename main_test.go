package main

import "testing"

func Test_getTestsFromDir(t *testing.T) {
	out, err := getTestsFromDir("/Users/dlaird/projects", false)
	if err != nil {
		t.Fatal(err)
	}

	_ = out
}

func Benchmark_getTestsFromDir(t *testing.B) {
	for i := 0; i < t.N; i++ {
		out, err := getTestsFromDir(".", false)
		if err != nil {
			t.Fatal(err)
		}

		_ = out
	}
}
