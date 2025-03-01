package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/MordFustang21/gotest/pkg/flamegraph"
	"github.com/manifoldco/promptui"
)

var (
	flagSet = flag.NewFlagSet("", flag.ContinueOnError)
	// Flags for the program
	subtest           = flagSet.Bool("s", false, "Run a specific subtest")
	verbose           = flagSet.Bool("verbose", false, "Print verbose output")
	debug             = flagSet.Bool("d", false, "Run test in debug mode with delve")
	quiet             = flagSet.Bool("q", false, "Disables verbose output on go test")
	runFromHistory    = flagSet.Bool("his", false, "Run a specific command from the history")
	rerun             = flagSet.Bool("r", false, "Re-run the last test")
	benchmark         = flagSet.Bool("b", false, "Run a specific benchmark.")
	withCoverage      = flagSet.Bool("cover", false, "Run the test with coverage and auto launch the viewer")
	withCPUProfile    = flagSet.Bool("cpu", false, "Run the test with a CPU profile")
	withMemoryProfile = flagSet.Bool("mem", false, "Run the test with a memory profile")
)

func main() {
	flagSet.Parse(os.Args[1:])
	err := loadConfig(globalConfig, WithDefaultConfig())
	if err != nil {
		panic(err)
	}

	err = run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	readDir := flag.Arg(0)
	if readDir == "" {
		readDir, _ = os.Getwd()
	}

	switch {
	case *subtest:
		availableTests, err := getTestsFromDir(readDir, false)
		if err != nil {
			return fmt.Errorf("error getting tests: %w", err)
		}

		if len(availableTests) == 0 {
			fmt.Println("No tests found in the directory")
			return nil
		}

		// select a test file and testToRun
		testToRun := selectTest(availableTests)

		// execute the test
		cmd, pass := executeTests(testToRun)
		logRunHistory(cmd, pass)

	case *rerun:
		he, err := getLastCommand()
		if err != nil {
			return fmt.Errorf("error getting last command: %w", err)
		}

		runHistoryEntry(he)

	case *benchmark:
		benchmarks, err := getTestsFromDir(readDir, true)
		if err != nil {
			return fmt.Errorf("error getting benchmarks: %w", err)
		}

		if len(benchmarks) == 0 {
			fmt.Println("No benchmarks found in the directory")
			return nil
		}

		selected := selectTest(benchmarks)
		runBenchmark(selected)

	case *runFromHistory:
		he, err := selectHistory()
		if err != nil {
			return fmt.Errorf("error selecting history: %w", err)
		}

		runHistoryEntry(he)

	default:
		// run a test for the directory
		cmd, pass := executeTests(Test{File: readDir})
		logRunHistory(cmd, pass)

	}

	return nil
}

func selectTest(availableTests []Test) Test {
	subtestPrompt := promptui.Select{
		Label: "Select a subtest",
		Items: availableTests,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ .File }}",
			Active:   "> {{ .Name }}",
			Inactive: "  {{ .Name }}",
			Selected: "{{ .Name }}",
		},
		Searcher: func(input string, index int) bool {
			test := availableTests[index]
			return strings.Contains(strings.ToLower(test.Name), strings.ToLower(input))
		},
	}

	index, _, err := subtestPrompt.Run()
	switch {
	case err == nil:
		return availableTests[index]
	case err == promptui.ErrInterrupt:
		fmt.Println("No Test Selected")
		os.Exit(0)
	default:
		panic(err)
	}

	return Test{}
}

// executeTests will run the test and return the command and if it passed.
func executeTests(t Test) (exec.Cmd, bool) {
	path, modRoot := testToPathAndRoot(t)

	args := []string{"test", quietMode(), path}
	if t.Name != "" {
		args = append(args, "-run", t.Name)
	}

	if *debug {
		return debugTest(t, path, modRoot)
	}

	var coverFile string
	if *withCoverage {
		tempFile, err := os.CreateTemp("", "go-test_"+t.Name)
		if err != nil {
			panic(err)
		}

		coverFile = tempFile.Name()
		tempFile.Close()

		args = append(args, "-coverprofile", coverFile)
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

	var outputWriter io.Writer = os.Stdout
	if globalConfig.ColorizeOutput {
		var colorReader io.Reader
		colorReader, outputWriter = io.Pipe()
		go colorizeOutput(colorReader)
	}

	cmd := exec.Cmd{
		Path:   p,
		Env:    os.Environ(),
		Args:   append([]string{"go"}, args...),
		Dir:    modRoot,
		Stdout: outputWriter,
		Stderr: os.Stderr,
	}

	fmt.Println("Running", cmd.Args, "@", cmd.Dir)

	var pass bool
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		pass = true
	case errors.As(err, &exit):
		// do nothing
	default:
		panic(err)
	}

	// if coverage was enabled launch the UI to view it
	if *withCoverage {
		cmd := exec.Cmd{
			Path:   p,
			Env:    os.Environ(),
			Dir:    modRoot,
			Args:   []string{"go", "tool", "cover", "-html=" + coverFile},
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}

		err = cmd.Run()
		if err != nil {
			panic(err)
		}
	}

	if *withCPUProfile {
		fmt.Println("Wrote CPU Profile to:", cpuProfile)
		// Generate the flamegraph
		svgData, err := flamegraph.GenerateFlamegraph(cpuProfile)
		if err != nil {
			panic(err)
		}

		// Write the flamegraph to a file
		flamegraphFile, err := os.CreateTemp("", "go-test_"+t.Name+".svg")
		if err != nil {
			panic(err)
		}

		_, err = flamegraphFile.Write(svgData)
		if err != nil {
			panic(err)
		}

		// Open the flamegraph in the browser
		cmd := exec.Command("open", flamegraphFile.Name())
		err = cmd.Run()
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

	return cmd, pass
}

func colorizeOutput(in io.Reader) {
	rdr := bufio.NewReader(in)
	for {
		line, err := rdr.ReadString('\n')
		if err != nil {
			panic(err)
		}

		switch {
		case strings.Contains(line, "FAIL"):
			fmt.Print("\033[31m")
		case strings.Contains(line, "PASS"):
			fmt.Print("\033[32m")
		}

		fmt.Print(line)
		fmt.Print("\033[0m")
	}
}

// quietMode will return a string that can be used to suppress output.
func quietMode() string {
	if *quiet {
		return ""
	}

	return "-v"
}
