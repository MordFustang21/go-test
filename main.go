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
	subtest        bool
	debug          bool
	runFromHistory bool
	rerun          bool

	// extra test features
	withCoverage      bool
	withCPUProfile    bool
	withMemoryProfile bool
)

func main() {
	flag.BoolVar(&rerun, "r", false, "Re-run the last test")
	flag.BoolVar(&subtest, "s", false, "Run a specific subtest.")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode.")
	flag.BoolVar(&runFromHistory, "his", false, "Run a specific command from the history")
	flag.BoolVar(&withCoverage, "cover", false, "Run the test with coverage and auto launch the viewer")
	flag.BoolVar(&withCPUProfile, "cpu", false, "Run the test with a CPU profile")
	flag.BoolVar(&withMemoryProfile, "mem", false, "Run the test with a memory profile")
	flag.Parse()

	readDir := flag.Arg(0)
	if readDir == "" {
		readDir, _ = os.Getwd()
	}

	switch {
	case runFromHistory:
		// todo: show history and select an option
		he := selectHistory()
		runHistoryEntry(he)
	case rerun:
		he := getLastCommand()
		runHistoryEntry(he)
	case subtest:
		availableTests := getTestsFromDir(readDir)
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
}

func getTestsFromDir(dir string) []Test {
	availableTests := []Test{}

	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		// if the file isn't a _test.go file, skip it
		if filepath.Ext(path) != "go" && !strings.Contains(filepath.Base(path), "_test") {
			return nil
		}

		// load in AST of the file and find test functions
		testFuncs := loadAST(path)
		availableTests = append(availableTests, testFuncs...)

		return nil
	})
	if err != nil {
		panic(err)
	}

	return availableTests
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
	path := t.File

	// if this is a file and not a directory, use the directory of the file
	if filepath.Ext(t.File) != "" {
		path = filepath.Dir(t.File)
	}

	modRoot := lookupModuleRoot(path)

	if modRoot == "" {
		panic("could not find module root")
	}

	// convert path to a module path
	path, _ = filepath.Rel(modRoot, path)
	path = "./" + path

	// in the event the directory is the root of the module, we need to add an extra ".." to
	// tell go test to recursively run all tests
	if path == "./." {
		path += ".."
	}

	args := []string{"test", "-v", path}
	if t.Name != "" {
		args = append(args, "-run", t.Name)
	}

	var coverFile string
	if withCoverage {
		tempFile, err := os.CreateTemp("", "go-test_"+t.Name)
		if err != nil {
			panic(err)
		}

		coverFile = tempFile.Name()
		tempFile.Close()

		args = append(args, "-coverprofile", coverFile)
	}

	var cpuProfile string
	if withCPUProfile {
		tempFile, err := os.CreateTemp("", "go-test_"+t.Name)
		if err != nil {
			panic(err)
		}

		coverFile = tempFile.Name()
		tempFile.Close()

		args = append(args, "-cpuprofile", cpuProfile)
	}

	var memoryProfile string
	if withCPUProfile {
		tempFile, err := os.CreateTemp("", "go-test_"+t.Name)
		if err != nil {
			panic(err)
		}

		coverFile = tempFile.Name()
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
	switch {
	case err == nil:
	// do nothing
	case errors.Is(err, &exec.ExitError{}):
	// do nothing
	default:
		panic(err)
	}
	if err != nil {
		panic(err)
	}

	// if coverage was enabled launch the UI to view it
	if withCoverage {
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

	return cmd
}

func lookupModuleRoot(path string) string {
	// start at end and work backwards to find the go.mod file
	for {
		if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
			return path
		}

		path = filepath.Dir(path)

		if path == "/" {
			break
		}
	}

	return ""
}

// Test represents a test case and the file it is in.
type Test struct {
	File string
	Name string
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
			if debug {
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
