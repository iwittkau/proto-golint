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
		lastPos token.Pos
	)

	spector.Preorder(nodeFilter, func(n ast.Node) {
		var (
			oldExpr, newExpr string
			pos, end         token.Pos
		)

		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				ignores[lhs.Pos()] = struct{}{}
			}

		case *ast.CallExpr:
			f, ok := x.Fun.(*ast.SelectorExpr)
			if !ok || !isProtoMessage(pass, f.X) {
				for _, arg := range x.Args {
					var a *ast.UnaryExpr
					a, ok = arg.(*ast.UnaryExpr)
					if !ok || a.Op != token.AND {
						continue
					}

					ignores[a.X.Pos()] = struct{}{}
				}

				ignores[x.Pos()] = struct{}{}
				return
			}

			oldExpr, newExpr, pos, end = makeFromCallAndSelectorExpr(x)
			if oldExpr == "" || newExpr == "" {
				ignores[x.Pos()] = struct{}{}
				return
			}

		case *ast.SelectorExpr:
			if !isProtoMessage(pass, x.X) {
				return
			}

			oldExpr, newExpr, pos, end = makeFromSelectorExpr(x)
		}

		if _, ok := ignores[n.Pos()]; ok {
			return
		}

		if oldExpr == "" || newExpr == "" || lastPos == pos {
			return
		}
		lastPos = pos

		if oldExpr == newExpr {
			return
		}

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

func makeFromCallAndSelectorExpr(expr *ast.CallExpr) (oldExpr, newExpr string, pos, end token.Pos) {
	pos = expr.Pos()
	end = expr.End()

	switch f := expr.Fun.(type) {
	case *ast.SelectorExpr:
		oldExpr, newExpr = handleExpr(nil, f.X)
		if oldExpr == "" || newExpr == "" {
			return "", "", 0, 0
		}

		oldExpr = fmt.Sprintf("%s.%s()", oldExpr, f.Sel.Name)
		newExpr = fmt.Sprintf("%s.%s()", newExpr, f.Sel.Name)

	default:
		fmt.Printf("makeFromCallExpr: not implemented for type: %s\n", reflect.TypeOf(f))
	}

	return oldExpr, newExpr, pos, end
}

func makeFromSelectorExpr(expr *ast.SelectorExpr) (oldExpr, newExpr string, pos, end token.Pos) {
	pos = expr.Pos()
	end = expr.End()

	oldExpr, newExpr = handleExpr(expr, expr.X)
	return oldExpr, newExpr, pos, end
}

func handleExpr(base, child ast.Expr) (newExpr, oldExpr string) {
	switch c := child.(type) {
	case *ast.Ident:
		oldExpr, newExpr = handleIdent(base, c)

	case *ast.SelectorExpr:
		oldExpr, newExpr = handleSelectorExpr(base, c)

	case *ast.IndexExpr:
		oldExpr, newExpr = handleIndexExpr(base, c)

	case *ast.CallExpr:
		oldExpr, newExpr = handleCallExpr(base, c)

	default:
		fmt.Printf("handleExpr: not implemented for type: %s\n", reflect.TypeOf(c))
	}

	return oldExpr, newExpr

}

func handleIdent(base ast.Expr, c *ast.Ident) (oldExpr, newExpr string) {
	if base == nil {
		return "", ""
	}

	switch b := base.(type) {
	case *ast.SelectorExpr:
		oldExpr = fmt.Sprintf("%s.%s", c.Name, b.Sel.Name)
		newExpr = fmt.Sprintf("%s.Get%s()", c.Name, b.Sel.Name)

	case *ast.IndexExpr:
		var index string
		switch i := b.Index.(type) {
		case *ast.BasicLit:
			index = i.Value

		case *ast.Ident:
			index = i.Name

		default:
			fmt.Printf("handleIdent: base is IndexExpr: not implemented for type: %s\n", reflect.TypeOf(i))
		}

		oldExpr = fmt.Sprintf("%s[%s]", c.Name, index)
		newExpr = fmt.Sprintf("%s[%s]", c.Name, index)

	default:
		fmt.Printf("handleIdent: not implemented for type: %s\n", reflect.TypeOf(b))
	}

	return oldExpr, newExpr
}

func handleSelectorExpr(base ast.Expr, c *ast.SelectorExpr) (oldExpr, newExpr string) {
	oldExpr, newExpr = handleExpr(c, c.X)

	if base == nil {
		return oldExpr, newExpr
	}

	switch b := base.(type) {
	case *ast.SelectorExpr:
		oldExpr = fmt.Sprintf("%s.%s", oldExpr, b.Sel.Name)
		newExpr = fmt.Sprintf("%s.Get%s()", newExpr, b.Sel.Name)

	case *ast.CallExpr:
		// skip

	default:
		fmt.Printf("handleSelectorExpr: not implemented for type: %s\n", reflect.TypeOf(b))
	}

	return oldExpr, newExpr
}

func handleCallExpr(base ast.Expr, c *ast.CallExpr) (newExpr, oldExpr string) {
	v, ok := c.Fun.(*ast.SelectorExpr)
	if ok {
		oldExpr, newExpr = handleExpr(c, v)
		oldExpr = strings.ReplaceAll(oldExpr, "GetGet", "Get")
		newExpr = strings.ReplaceAll(newExpr, "GetGet", "Get")
	}

	if base == nil {
		return oldExpr + "()", newExpr
	}

	switch b := base.(type) {
	case *ast.SelectorExpr:
		oldExpr = fmt.Sprintf("%s().%s", oldExpr, b.Sel.Name)
		newExpr = fmt.Sprintf("%s.Get%s()", newExpr, b.Sel.Name)

	default:
		fmt.Printf("handleCallExpr: not implemented for type: %s\n", reflect.TypeOf(b))
	}

	return oldExpr, newExpr
}

func handleIndexExpr(base ast.Expr, c *ast.IndexExpr) (oldExpr, newExpr string) {
	oldExpr, newExpr = handleExpr(c, c.X)

	if base == nil {
		return oldExpr, newExpr
	}

	switch b := base.(type) {
	case *ast.SelectorExpr:
		oldExpr = fmt.Sprintf("%s.%s", oldExpr, b.Sel.Name)
		newExpr = fmt.Sprintf("%s.Get%s()", newExpr, b.Sel.Name)

	default:
		fmt.Printf("handleIndexExpr: not implemented for type: %s\n", reflect.TypeOf(b))
	}

	return oldExpr, newExpr
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
