package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var ProtoGetters = &analysis.Analyzer{
	Name:     "getters",
	Doc:      "reports direct reads from proto message fields when getters should be used",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	var (
		spector    = pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
		nodeFilter = []ast.Node{
			(*ast.SelectorExpr)(nil),
			(*ast.AssignStmt)(nil),
			(*ast.CallExpr)(nil),
		}
		ignores = map[token.Pos]struct{}{}
	)

	spector.Preorder(nodeFilter, func(n ast.Node) {
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				ignores[lhs.Pos()] = struct{}{}
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			ignores[call.Pos()] = struct{}{}
			return
		}
		if _, ok := ignores[n.Pos()]; ok {
			return
		}
		be := n.(*ast.SelectorExpr)
		if !isProtoMessage(pass, be.X) {
			return
		}

		makeReport(pass, be)
	})
	return nil, nil
}

func makeReport(pass *analysis.Pass, be *ast.SelectorExpr) {

	/*
		x, ok := be.X.(*ast.Ident)
		if ok {
			oldPrefix = x.Name
			newPrefix = x.Name
		}

		be2 := be
		for {
			be2, ok = be2.X.(*ast.SelectorExpr)
			if ok {
				if oldPrefix == "" {
					oldPrefix = fmt.Sprint(be2.X)
					newPrefix = fmt.Sprint(be2.X)
				}

				oldPrefix = fmt.Sprintf("%s.%s", oldPrefix, be2.Sel.Name)
				newPrefix = fmt.Sprintf("%s.Get%s()", newPrefix, be2.Sel.Name)
				continue
			}

			break
		}
	*/

	oldExpr, newExpr, tok := makeExpr(be)
	if oldExpr == "" || newExpr == "" {
		return
	}

	pass.Report(analysis.Diagnostic{
		Pos:     tok.Pos(),
		End:     tok.End(),
		Message: fmt.Sprintf(`proto message field read without getter: %s should be %s`, oldExpr, newExpr),
		SuggestedFixes: []analysis.SuggestedFix{
			{
				Message: fmt.Sprintf("should replace `%s` with `%s`", oldExpr, newExpr),
				TextEdits: []analysis.TextEdit{
					{
						Pos:     tok.Pos(),
						End:     tok.End(),
						NewText: []byte(newExpr),
					},
				},
			},
		},
	})
}

func makeExpr(expr *ast.SelectorExpr) (string, string, ast.Node) {
	var oldExpr, newExpr string

loop:
	for {
		switch x := expr.X.(type) {
		case *ast.Ident:
			if oldExpr != "" {
				break loop
			}

			oldExpr = fmt.Sprintf("%s.%s", x.Name, expr.Sel.Name)
			newExpr = fmt.Sprintf("%s.Get%s()", x.Name, expr.Sel.Name)
			break loop

		case *ast.SelectorExpr:
			oldExpr, newExpr, _ = makeExpr(x)

			if oldExpr == "" {
				oldExpr = fmt.Sprint(x)
				newExpr = fmt.Sprint(x)
			}

			oldExpr = fmt.Sprintf("%s.%s", oldExpr, expr.Sel.Name)
			newExpr = fmt.Sprintf("%s.Get%s()", newExpr, expr.Sel.Name)

			vv, ok := x.X.(*ast.SelectorExpr)
			if !ok {
				break loop
			}
			expr = vv

		default:
			fmt.Printf("Not implemented for type: %s\n", reflect.TypeOf(x))
			break loop
		}
	}

	return oldExpr, newExpr, expr
}

const messageState = "google.golang.org/protobuf/internal/impl.MessageState"

func isProtoMessage(pass *analysis.Pass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	ptr, ok := t.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	sct, ok := named.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	if sct.NumFields() == 0 {
		return false
	}

	return sct.Field(0).Type().String() == messageState
}
