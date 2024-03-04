package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
)

var (
	// Flags for the program
	subtest        = flag.Bool("s", false, "Run a specific subtest")
	debug          = flag.Bool("debug", false, "Enable debug mode")
	runFromHistory = flag.Bool("his", false, "Run a specific command from the history")
	rerun          = flag.Bool("r", false, "Re-run the last test")

	// extra test features
	benchmark         = flag.Bool("b", false, "Run a specific benchmark.")
	withCoverage      = flag.Bool("cover", false, "Run the test with coverage and auto launch the viewer")
	withCPUProfile    = flag.Bool("cpu", false, "Run the test with a CPU profile")
	withMemoryProfile = flag.Bool("mem", false, "Run the test with a memory profile")
)

func main() {
	flag.Parse()
	err := run()
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
	case *benchmark:
		// get benchmarks
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
		// todo: show history and select an option
		he, err := selectHistory()
		if err != nil {
			return fmt.Errorf("error selecting history: %w", err)
		}

		runHistoryEntry(he)

	case *rerun:
		he, err := getLastCommand()
		if err != nil {
			return fmt.Errorf("error getting last command: %w", err)
		}

		runHistoryEntry(he)

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
		cmd := executeTests(testToRun)
		logRunHistory(cmd)
	default:
		// run a test for the directory
		cmd := executeTests(Test{File: readDir})
		logRunHistory(cmd)

	}

	return nil
}

func getTestsFromDir(dir string, benchmarks bool) ([]Test, error) {
	availableTests := []Test{}

	err := filepath.WalkDir(dir, func(path string, info fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// if the file isn't a _test.go file, skip it
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// load in AST of the file and find test functions
		if benchmarks {
			testFuncs := findBenchmarks(path)
			availableTests = append(availableTests, testFuncs...)
		} else {
			testFuncs := loadAST(path)
			availableTests = append(availableTests, testFuncs...)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking the path %s: %w", dir, err)
	}

	return availableTests, nil
}

func selectTest(availableTests []Test) Test {
	subtestPrompt := promptui.Select{
		Label: "Select a subtest",
		Items: availableTests,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ .File }}",
			Active:   "\U0001F449 {{ .Name }}",
			Inactive: "  {{ .Name }}",
			Selected: "{{ .Name }}",
		},
		Searcher: func(input string, index int) bool {
			test := availableTests[index]
			if strings.Contains(
				strings.ToLower(test.Name),
				strings.ToLower(input),
			) {
				return true
			}

			return false
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

func executeTests(t Test) exec.Cmd {
	path, modRoot := testToPathAndRoot(t)

	args := []string{"test", "-v", path}
	if t.Name != "" {
		args = append(args, "-run", t.Name)
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
	if *withCPUProfile {
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

	cmd := exec.Cmd{
		Path:   p,
		Env:    os.Environ(),
		Args:   append([]string{"go"}, args...),
		Dir:    modRoot,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	fmt.Println("Running", cmd.Args, "@", cmd.Dir)

	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
	// do nothing
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
			Args:   []string{"tool", "cover", "-html=" + coverFile},
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

	return cmd
}

// Test represents a test case and the file it is in.
type Test struct {
	File        string
	Name        string
	IsBenchmark bool
}

// loadAST loads the AST of a file and returns all test functions in the file.
func loadAST(path string) []Test {
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

			if strings.HasPrefix(x.Name.Name, "Test") && x.Name.Name != "TestMain" {
				var subtests []string

			testFunc:
				for _, item := range x.Body.List {
					switch stmt := item.(type) {
					case *ast.RangeStmt:
						// check body for call to t.Run
						for _, forItem := range stmt.Body.List {
							switch forT := forItem.(type) {
							case *ast.ExprStmt:
								switch c := forT.X.(type) {
								case *ast.CallExpr:
									// check if the call is to t.Run
									// if it is, add the subtest to the list of tests
									// if it isn't, continue
									f, ok := c.Fun.(*ast.SelectorExpr)
									if !ok {
										continue
									}

									if f.Sel.Name == "Run" {
										// we are inside the t.Run call we can lookup the test name from here
										// set the argName so we can look it up on the list of test structs

										// set args to the first argument of the t.Run call (the subtest name)
										argName := c.Args[0].(*ast.SelectorExpr).Sel.Name
										testData := stmt.X.(*ast.Ident).Obj.Decl.(*ast.AssignStmt).Rhs[0]

										for _, el := range testData.(*ast.CompositeLit).Elts {
											for _, structField := range el.(*ast.CompositeLit).Elts {
												field := structField.(*ast.KeyValueExpr)
												if field.Key.(*ast.Ident).Name == argName {
													testName := field.Value.(*ast.BasicLit).Value
													subtests = append(subtests, strings.ReplaceAll(testName, "\"", ""))
												}
											}
										}

										// all subtests have been added
										break testFunc
									}
								}
							}
						}
					}
				}

				// create entry to run the whole test function
				tests = append(tests, Test{
					File: path,
					Name: x.Name.Name,
				})

				// add in subtests if they exist
				for _, subtest := range subtests {
					tests = append(tests, Test{
						File: path,
						Name: x.Name.Name + "/" + subtest,
					})
				}
			}
		}

		return true
	})

	return tests
}
