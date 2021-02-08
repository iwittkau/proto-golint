package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

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

		oldExpr := fmt.Sprintf("%s.%s", be.X, be.Sel.Name)
		newExpr := fmt.Sprintf("%s.Get%s()", be.X, be.Sel.Name)
		pass.Report(analysis.Diagnostic{
			Pos:     be.Pos(),
			Message: fmt.Sprintf(`proto message field read without getter: %s.%s`, be.X, be.Sel.Name),
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: fmt.Sprintf("should replace `%s` with `%s`", oldExpr, newExpr),
					TextEdits: []analysis.TextEdit{
						{
							Pos:     be.Pos(),
							End:     be.End(),
							NewText: []byte(newExpr),
						},
					},
				},
			},
		})
	})
	return nil, nil
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
