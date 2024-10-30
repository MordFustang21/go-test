package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	dbg "runtime/debug"
	"strings"
)

// Test represents a test case and the file it is in.
type Test struct {
	File        string
	Name        string
	IsBenchmark bool
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
			testFuncs := findTests(path)
			availableTests = append(availableTests, testFuncs...)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking the path %s: %w", dir, err)
	}

	return availableTests, nil
}

// findTests loads the AST of a file and returns all test functions in the file.
func findTests(path string) []Test {
	var tests []Test

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	ast.Inspect(f, func(n ast.Node) bool {
		// in the event of a panic catch it and print debug information
		defer func() {
			if r := recover(); r != nil {
				if *verbose {
					fmt.Printf("Error in file %s : %s\n%s", path, r, dbg.Stack())
				}
			}
		}()

		switch x := n.(type) {
		case *ast.FuncDecl:
			if *verbose {
				fmt.Println("Evalutating", x.Name.Name)
			}

			// skip non test functions
			if !strings.HasPrefix(x.Name.Name, "Test") || x.Name.Name == "TestMain" {
				return true
			}

			// create root test entry
			tests = append(tests, Test{
				File: path,
				Name: x.Name.Name,
			})

			var subtests []string
			for _, item := range x.Body.List {
				subtests = append(subtests, astToTests(x.Name.Name, item)...)
			}

			// convert subtests into Test entries
			for _, subtest := range subtests {
				tests = append(tests, Test{
					File: path,
					Name: subtest,
				})
			}
		}

		return true
	})

	return tests
}

func astToTests(parentTestName string, item ast.Node) []string {
	defer func() {
		// don't fail out the whole ast due to one bad test
		if r := recover(); r != nil {
			if *verbose {
				fmt.Println("Error in astToTests", parentTestName, r)
			}
		}
	}()

	var subtests []string
	switch stmt := item.(type) {
	case *ast.ExprStmt:
		switch c := stmt.X.(type) {
		case *ast.CallExpr:
			// check if the call is to t.Run
			// if it is, add the subtest to the list of tests
			// if it isn't, continue
			f, ok := c.Fun.(*ast.SelectorExpr)
			if !ok {
				return nil
			}

			if f.Sel.Name != "Run" {
				return nil
			}

			// set args to the first argument of the t.Run call (the subtest name)
			// if the argument is a basic literal, IE a string add it to the list of subtests else look up the field name
			var testName string
			if v, ok := c.Args[0].(*ast.BasicLit); ok {
				testName = parentTestName + "/" + strings.ReplaceAll(v.Value, "\"", "")
				// create entry for this test before checking for subtests within closure
				subtests = append(subtests, testName)
			} else {
				// this is a table test we need to find all table entries
				// if the argument is a composite literal, we have to look up the field name
				fieldName := c.Args[0].(*ast.SelectorExpr).Sel.Name
				tableTests := findTestNameInTable(c.Args[0].(*ast.SelectorExpr), fieldName)
				for _, tableTest := range tableTests {
					subtests = append(subtests, parentTestName+"/"+tableTest)
				}
			}

			// check right hand side of assignment for function body
			if f, ok := c.Args[1].(*ast.FuncLit); ok {
				for _, fItem := range f.Body.List {
					subtests = append(subtests, astToTests(testName, fItem)...)
				}
			}
		}

	// found range statement, check for t.Run calls in the body for subtests
	case *ast.RangeStmt:
		// check body for call to t.Run
		for _, forItem := range stmt.Body.List {
			subtests = append(subtests, astToTests(parentTestName, forItem)...)
		}
	}

	return subtests
}

func findTestNameInTable(stmt *ast.SelectorExpr, fieldName string) []string {
	var subtests []string

	testData := stmt.X.(*ast.Ident).Obj.Decl.(*ast.AssignStmt).Rhs[0]
	testData = testData.(*ast.UnaryExpr).X.(*ast.Ident).Obj.Decl.(*ast.AssignStmt).Rhs[0]

	for _, el := range testData.(*ast.CompositeLit).Elts {
		lit := el.(*ast.CompositeLit)
		for _, structField := range lit.Elts {
			switch t := structField.(type) {
			case *ast.BasicLit:
				// this struct isn't keyed so we have to look up parent struct and compare the field name
				subtests = append(subtests, strings.Trim(t.Value, `"`))
			case *ast.KeyValueExpr:
				if t.Key.(*ast.Ident).Name == fieldName {
					subtests = append(subtests, strings.Trim(t.Value.(*ast.BasicLit).Value, `"`))
				}
			case *ast.FuncLit:
			// if the subtest is a function literal, we can't get the name from the AST
			case *ast.CompositeLit:
			default:
				panic(fmt.Sprintf("Unknown type %T @ %d", t, structField.Pos()))
			}
		}
	}

	return subtests
}
