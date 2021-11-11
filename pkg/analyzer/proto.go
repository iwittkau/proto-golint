package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strings"

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

	var lastPos token.Pos
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

		oldExpr, newExpr, pos, end := makeExpr(be)
		if oldExpr == "" || newExpr == "" || lastPos == pos {
			return
		}
		lastPos = pos

		pass.Report(analysis.Diagnostic{
			Pos:     pos,
			End:     end,
			Message: fmt.Sprintf(`proto message field read without getter: %q should be %q`, oldExpr, newExpr),
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: fmt.Sprintf("should replace %q with %q", oldExpr, newExpr),
					TextEdits: []analysis.TextEdit{
						{
							Pos:     pos,
							End:     end,
							NewText: []byte(newExpr),
						},
					},
				},
			},
		})
	})
	return nil, nil
}

func makeExpr(expr *ast.SelectorExpr) (oldExpr, newExpr string, pos, end token.Pos) {
	pos = expr.Pos()
	end = expr.End()

loop:
	for {
		var end2 token.Pos

		switch x := expr.X.(type) {
		case *ast.Ident:
			if oldExpr != "" {
				break loop
			}

			oldExpr = fmt.Sprintf("%s.%s", x.Name, expr.Sel.Name)
			newExpr = fmt.Sprintf("%s.Get%s()", x.Name, expr.Sel.Name)
			break loop

		case *ast.SelectorExpr:
			oldExpr, newExpr, _, end2 = makeExpr(x)
			if end2 > end {
				end = end2
			}

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

		case *ast.CallExpr:
			v, ok := x.Fun.(*ast.SelectorExpr)
			if ok {
				oldExpr, newExpr, _, end2 = makeExpr(v)
				if end2 > end {
					end = end2
				}

				oldExpr = strings.ReplaceAll(oldExpr, "GetGet", "Get")
				newExpr = strings.ReplaceAll(newExpr, "GetGet", "Get")
			}

			oldExpr = fmt.Sprintf("%s.%s", oldExpr, expr.Sel.Name)
			newExpr = fmt.Sprintf("%s.Get%s()", newExpr, expr.Sel.Name)
			break loop

		default:
			fmt.Printf("Not implemented for type: %s\n", reflect.TypeOf(x))
			break loop
		}
	}

	return oldExpr, newExpr, pos, end
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
