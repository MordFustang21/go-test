package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	dbg "runtime/debug"
	"strings"
)

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
				if *debug {
					fmt.Printf("Error in file %s : %s\n%s", path, r, dbg.Stack())
				}
			}
		}()

		switch x := n.(type) {
		case *ast.FuncDecl:
			if *debug {
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
			} else {
				// if the argument is a composite literal, we have to look up the field name
				fieldName := c.Args[0].(*ast.SelectorExpr).Sel.Name
				testData := stmt.X.(*ast.Ident).Obj.Decl.(*ast.AssignStmt).Rhs[0]

				for _, el := range testData.(*ast.CompositeLit).Elts {
					lit := el.(*ast.CompositeLit)
					for _, structField := range lit.Elts {
						switch t := structField.(type) {
						case *ast.BasicLit:
							// this struct isn't keyed so we have to look up parent struct and compare the field name
							testName = parentTestName + "/" + strings.ReplaceAll(t.Value, "\"", "")
							break
						case *ast.KeyValueExpr:
							if t.Key.(*ast.Ident).Name == fieldName {
								testName = parentTestName + "/" + strings.ReplaceAll(t.Value.(*ast.BasicLit).Value, "\"", "")
								break
							}
						case *ast.FuncLit:
						// if the subtest is a function literal, we can't get the name from the AST
						case *ast.CompositeLit:
						default:
							panic(fmt.Sprintf("Unknown type %T @ %d", t, structField.Pos()))
						}
					}
				}
			}

			// create entry for this test before checking for subtests within closure
			subtests = append(subtests, testName)

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
