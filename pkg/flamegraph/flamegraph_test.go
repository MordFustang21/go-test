package flamegraph

import (
	"os"
	"testing"
)

func Test_generateFlameGraph(t *testing.T) {
	out, err := GenerateFlamegraph("testdata/go-test_Benchmark_findTests3094592916")
	if err != nil {
		t.Fatal(err)
	}

	// The HTML output is not deterministic, so we can't compare it directly. Instead, we'll just check that it's not empty.
	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}
}

func Test_profileToRaw(t *testing.T) {
	raw, err := profileToRaw("testdata/go-test_Benchmark_findTests3094592916")
	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("testdata/raw.txt")
	if err != nil {
		t.Fatal(err)
	}

	if string(raw) != string(expected) {
		t.Fatalf("expected %s, got %s", string(expected), string(raw))
	}
}

func Test_foldRaw(t *testing.T) {
	input, err := os.ReadFile("testdata/raw.txt")
	if err != nil {
		t.Fatal(err)
	}

	out, err := foldRaw(input)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("testdata/out.folded")
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != string(expected) {
		t.Fatalf("expected %s, got %s", string(expected), string(out))
	}
}
