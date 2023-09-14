package envreport

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"golang.org/x/tools/go/analysis"
)

type DeclarationMap map[string]map[ast.Node]struct{}

var dump bool

var Analyzer = &analysis.Analyzer{
	Name: "envreport",
	Doc:  "find requests for environment variables to add to readme",
	Run:  run,
}

func init() {
	Analyzer.Flags.BoolVar(&dump, "dump", false, "Set this value if you'd like to dump the tokens found. Generally used for debugging")
}

func run(pass *analysis.Pass) (any, error) {
	getenvCalls := make([]*ast.CallExpr, 0)
	declMap := make(DeclarationMap)

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if dump {
				spew.Dump(n)
				return true
			}

			switch node := n.(type) {
			case *ast.GenDecl:
				captureDeclaration(node, declMap)

			case *ast.AssignStmt:
				associateAssignment(node, declMap)

			case *ast.CallExpr:
				if foundGetenv(node) {
					getenvCalls = append(getenvCalls, node)
				}
			}

			return true
		})

	}

	valMap := make(map[*ast.BasicLit]struct{})

	for _, call := range getenvCalls {
		switch v := call.Args[0].(type) {
		case *ast.BasicLit:
			valMap[v] = struct{}{}

		case *ast.Ident:
			declNodeMap := declMap[v.Name]

			for node := range declNodeMap {
				switch v := node.(type) {
				case *ast.BasicLit:
					valMap[v] = struct{}{}

				case *ast.Ident:
					valMap[getLitForIdent(v, declMap)] = struct{}{}
				}
				break
			}

		default:
			panic("TODO: capture value if a function like fmt.Sprintf is used to generate the env var")
		}
	}

	for value := range valMap {
		fmt.Println(strings.Trim(value.Value, "\""))
	}

	return nil, nil
}

func getLitForIdent(ident *ast.Ident, declMap DeclarationMap) *ast.BasicLit {
	nodeMap, found := declMap[ident.Name]
	if !found {
		return nil
	}

	for node := range nodeMap {
		switch v := node.(type) {
		case *ast.BasicLit:
			return v

		case *ast.Ident:
			return getLitForIdent(v, declMap)

		}

		break
	}

	return nil
}

func associateAssignment(asSt *ast.AssignStmt, declMap DeclarationMap) {
	if len(asSt.Lhs) == 0 {
		return
	}

	variable, ok := asSt.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}

	// not returning a value to be used later to ensure the mutations persist
	if _, found := declMap[variable.Name]; !found {
		return
	}

	if declMap[variable.Name] == nil {
		declMap[variable.Name] = make(map[ast.Node]struct{})
	}

	switch v := asSt.Rhs[0].(type) {
	case *ast.BasicLit:
		declMap[variable.Name][v] = struct{}{}

	case *ast.Ident:
		declMap[variable.Name][getLitForIdent(v, declMap)] = struct{}{}
	}
}

func foundGetenv(funcCall *ast.CallExpr) bool {
	if funcDecl, ok := funcCall.Fun.(*ast.SelectorExpr); ok {
		pkg, ok := funcDecl.X.(*ast.Ident)
		if !ok {
			return false
		}

		pkgName := pkg.Name
		funcName := funcDecl.Sel.Name

		if pkgName == "os" && funcName == "Getenv" {
			return true
		}
	}

	return false
}

func captureDeclaration(decl *ast.GenDecl, declMap DeclarationMap) {
	var ident *ast.Ident
	var identVal ast.Node

	valSpec, ok := decl.Specs[0].(*ast.ValueSpec)
	if !ok {
		return
	}

	ident = valSpec.Names[0]

	// for example: var x
	// This is a declaration where the identifier is set, but there
	// are no values. In this case, just store the identifier, and
	// move on.
	if valSpec.Values == nil {
		declMap[ident.Name] = make(map[ast.Node]struct{})
		return
	}

	switch v := valSpec.Values[0].(type) {
	case *ast.BasicLit, *ast.Ident:
		identVal = v
	default:
		identVal = &ast.BasicLit{
			Value: fmt.Sprintf(
				"expected identifier (%s) value to be either *ast.BasicLit or *ast.Ident, but got %s",
				ident.Name,
				reflect.TypeOf(v),
			),
		}
	}

	if declMap[ident.Name] == nil {
		declMap[ident.Name] = make(map[ast.Node]struct{})
	}

	declMap[ident.Name][identVal] = struct{}{}
}
