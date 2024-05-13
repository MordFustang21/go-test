package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	bolt "go.etcd.io/bbolt"
)

func runBenchmark(t Test) {
	// create base args with verbose and a run that filters tests so we only run benchmarks
	// -run XXX skips tests so it's just benchmarks
	args := []string{"test", "-v", "-run", "XXX", "-bench"}
	if t.Name != "" {
		args = append(args, t.Name)
	} else {
		args = append(args, ".")
	}

	if *benchMem {
		args = append(args, "-benchmem")
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

	_, modRoot := testToPathAndRoot(t)

	// create a buffer to capture the output of the benchmark
	benchBuffer := &bytes.Buffer{}
	w := io.MultiWriter(os.Stdout, benchBuffer)

	cmd := exec.Cmd{
		Path:   p,
		Env:    os.Environ(),
		Args:   append([]string{"go"}, args...),
		Dir:    modRoot,
		Stdout: w,
		Stderr: os.Stderr,
	}

	fmt.Println("Running", cmd.Args, "@", cmd.Dir)

	err = cmd.Run()
	switch {
	case err == nil:
		// store the successful benchmark
		storeBenchmarkResult(t, cmd, benchBuffer)
	case errors.Is(err, &exec.ExitError{}):
	// do nothing
	default:
		panic(err)
	}
	if err != nil {
		panic(err)
	}

	if *withCPUProfile {
		fmt.Println("Wrote CPU Profile to:", cpuProfile)
		// show top methods
		cmd := exec.Command("go", "tool", "pprof", "-top", cpuProfile)
		cmd.Stdout = os.Stdout
		cmd.Run()
	}

	if *withMemoryProfile {
		fmt.Println("Wrote Memory Profile to:", memoryProfile)
		cmd := exec.Command("go", "tool", "pprof", "-top", memoryProfile)
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

const benchmarkDB = "benchmarks.db"

func storeBenchmarkResult(t Test, cmd exec.Cmd, benchBuffer *bytes.Buffer) {
	db := getDBInstance(benchmarkDB)

	// store the benchmark under the test name appending a new entry, otherwise store it under the directory.
	var pastResults map[time.Time][]byte
	benchmarkKey := t.Name
	if t.Name == "" {
		benchmarkKey = cmd.Dir
	}

	db.Update(func(tx *bolt.Tx) error {
		bk := tx.Bucket([]byte("benchmarks"))
		if bk == nil {
			var err error
			bk, err = tx.CreateBucket([]byte("benchmarks"))
			if err != nil {
				panic(err)
			}
		}

		v := bk.Get([]byte(benchmarkKey))
		if v != nil {
			// append to the existing entry
			err := json.Unmarshal(v, &pastResults)
			if err != nil {
				panic(err)
			}
		}

		if pastResults == nil {
			pastResults = make(map[time.Time][]byte)
		}

		pastResults[time.Now()] = benchBuffer.Bytes()

		jsonData, err := json.Marshal(pastResults)
		if err != nil {
			panic(err)
		}

		err = bk.Put([]byte(benchmarkKey), jsonData)
		if err != nil {
			panic(err)
		}

		return nil
	})
}

func getBenchmarkResults() map[string]map[time.Time][]byte {
	db := getDBInstance(benchmarkDB)
	results := make(map[string]map[time.Time][]byte)

	db.View(func(tx *bolt.Tx) error {
		bk := tx.Bucket([]byte("benchmarks"))
		bk.ForEach(func(k, v []byte) error {
			var testResults map[time.Time][]byte
			err := json.Unmarshal(v, &testResults)
			if err != nil {
				panic(err)
			}

			results[string(k)] = testResults

			return nil
		})

		return nil
	})

	return results
}

func selectBenchmarkResult(runs map[string]map[time.Time][]byte) []byte {
	// convert map into an array of options
	var runArray []string
	if len(runs) == 0 {
		fmt.Println("No benchmarks found")
		os.Exit(0)
	}

	for k := range runs {
		runArray = append(runArray, k)
	}

	subtestPrompt := promptui.Select{
		Label: "Select a benchmark to view",
		Items: runArray,
		Templates: &promptui.SelectTemplates{
			Label:  "{{ . }}",
			Active: "\U0001F449 {{ . }}",
		},
		Searcher: func(input string, index int) bool {
			test := runArray[index]
			if strings.Contains(strings.ToLower(test), strings.ToLower(input)) {
				return true
			}

			return false
		},
	}

	index, _, err := subtestPrompt.Run()
	switch {
	case err == nil:
		key := runArray[index]
		// build up a buffer of resulsts
		var buffer bytes.Buffer
		v, ok := runs[key]
		if !ok {
			return []byte("No runs found")
		}

		for k, v := range v {
			buffer.WriteString(fmt.Sprintf("Run at %s\n", k))
			buffer.Write(v)
		}

		return buffer.Bytes()

	case err == promptui.ErrInterrupt:
		fmt.Println("No benchmark Selected")
		os.Exit(0)
		return nil
	default:
		panic(err)
	}
}

func storeCPUProfile(t Test, profile string) {
	// store the profile in bbolt
	db := getDBInstance(benchmarkDB)
	profileContents, err := os.ReadFile(profile)
	if err != nil {
		panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("cpuProfile"))
		bucket.Put([]byte("todo-key"), profileContents)
		return nil
	})
	if err != nil {
		panic(err)
	}
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
			if *debug {
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
