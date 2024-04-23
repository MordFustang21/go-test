package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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
				fmt.Printf("Error in file %s : %s\n", path, r)
			}
		}()

		switch x := n.(type) {
		case *ast.FuncDecl:
			if *debug {
				fmt.Println("Evalutating", x.Name.Name)
			}

			if strings.HasPrefix(x.Name.Name, "Test") && x.Name.Name != "TestMain" {
				var subtests []string

				// create entry to run the whole test function
				tests = append(tests, Test{
					File: path,
					Name: x.Name.Name,
				})

			testFunc:
				for _, item := range x.Body.List {
					switch stmt := item.(type) {
					// look for range statement that's iterating over t.Run calls
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
										fieldName := c.Args[0].(*ast.SelectorExpr).Sel.Name
										testData := stmt.X.(*ast.Ident).Obj.Decl.(*ast.AssignStmt).Rhs[0]

										for _, el := range testData.(*ast.CompositeLit).Elts {
											lit := el.(*ast.CompositeLit)
											for _, structField := range lit.Elts {
												switch t := structField.(type) {
												case *ast.BasicLit:
													// this struct isn't keyed so we have to look up parent struct and compare the field name
													subtests = append(subtests, strings.ReplaceAll(t.Value, "\"", ""))
													break
												case *ast.KeyValueExpr:
													if t.Key.(*ast.Ident).Name == fieldName {
														testName := t.Value.(*ast.BasicLit).Value
														subtests = append(subtests, strings.ReplaceAll(testName, "\"", ""))
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

										// all subtests have been added
										break testFunc
									}
								}
							}
						}
					}
				}

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
