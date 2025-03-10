package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/MordFustang21/gotest/pkg/flamegraph"
	bolt "go.etcd.io/bbolt"
)

func runBenchmark(t Test) {
	path, modRoot := testToPathAndRoot(t)

	// create base args with verbose and a run that filters tests so we only run benchmarks
	args := []string{"test", "-v", path, "-run", "XXX"}
	if t.Name != "" {
		args = append(args, "-bench", t.Name)
	}

	var cpuProfile string
	if *withCPUProfile {
		tempFile, err := os.CreateTemp("", "go-test_"+t.Name)
		if err != nil {
			panic(err)
		}

		cpuProfile = tempFile.Name()
		tempFile.Close()

		args = append(args, "-cpuprofile", cpuProfile)
	}

	var memoryProfile string
	if *withMemoryProfile {
		tempFile, err := os.CreateTemp("", "go-test_"+t.Name)
		if err != nil {
			panic(err)
		}

		memoryProfile = tempFile.Name()
		tempFile.Close()

		args = append(args, "-memprofile", memoryProfile)
	}

	p, err := exec.LookPath("go")
	if err != nil {
		panic(err)
	}

	// create a buffer to capture the output of the benchmark
	benchBuffer := &bytes.Buffer{}
	writer := io.MultiWriter(os.Stdout, benchBuffer)

	cmd := exec.Cmd{
		Path:   p,
		Env:    os.Environ(),
		Args:   append([]string{"go"}, args...),
		Dir:    modRoot,
		Stdout: writer,
		Stderr: os.Stderr,
	}

	fmt.Println("Running", cmd.Args, "@", cmd.Dir)

	err = cmd.Run()
	switch {
	case err == nil:
		// store the successful benchmark
		storeBenchmarkResult(cmd, benchBuffer)
	case errors.Is(err, &exec.ExitError{}):
	// do nothing
	default:
		panic(err)
	}

	if *withCPUProfile {
		fmt.Println("Wrote CPU Profile to:", cpuProfile)
		// Generate the flamegraph
		svgData, err := flamegraph.GenerateFlamegraph(cpuProfile)
		if err != nil {
			panic(err)
		}

		err = flamegraph.ServeFlamegraph(svgData)
		if err != nil {
			panic(err)
		}
	}

	if *withMemoryProfile {
		fmt.Println("Wrote Memory Profile to:", memoryProfile)
		cmd := exec.Command("go", "tool", "pprof", "-top", memoryProfile)
		cmd.Stdout = os.Stdout
		err = cmd.Run()
		if err != nil {
			panic(err)
		}
	}
}

const benchmarkDB = "benchmarks.db"

func storeBenchmarkResult(cmd exec.Cmd, benchBuffer *bytes.Buffer) {
	db := getHistoryFile(benchmarkDB)

	db.Update(func(tx *bolt.Tx) error {
		return nil
	})
}

func findBenchmarks(path string) []Test {
	var tests []Test

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if *verbose {
				fmt.Println("Evalutating", x.Name.Name)
			}

			if strings.HasPrefix(x.Name.Name, "Benchmark") {
				// create entry to run the whole test function
				tests = append(tests, Test{
					File:        path,
					Name:        x.Name.Name,
					IsBenchmark: true,
				})
			}
		}

		return true
	})

	return tests
}
