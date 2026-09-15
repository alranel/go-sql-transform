package sqltransform

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/alranel/go-sql-transform/internal/extract"
	"github.com/alranel/go-sql-transform/internal/wrapwrite"
)

// Expr is an opaque SQL expression node used with WrapWriteExpressions.
type Expr struct {
	node *pg_query.Node
}

// Call builds a function-call expression: name(args...).
func Call(name string, args ...Expr) Expr {
	argNodes := make([]*pg_query.Node, len(args))
	for i, a := range args {
		argNodes[i] = a.node
	}
	return Expr{node: &pg_query.Node{
		Node: &pg_query.Node_FuncCall{
			FuncCall: &pg_query.FuncCall{
				Funcname: []*pg_query.Node{
					{
						Node: &pg_query.Node_String_{
							String_: &pg_query.String{Sval: name},
						},
					},
				},
				Args:       argNodes,
				Funcformat: pg_query.CoercionForm_COERCE_EXPLICIT_CALL,
			},
		},
	}}
}

// StringLiteral builds a string constant expression.
func StringLiteral(s string) Expr {
	return Expr{node: &pg_query.Node{
		Node: &pg_query.Node_AConst{
			AConst: &pg_query.A_Const{
				Val: &pg_query.A_Const_Sval{
					Sval: &pg_query.String{Sval: s},
				},
			},
		},
	}}
}

// IntLiteral builds an integer constant expression.
func IntLiteral(n int32) Expr {
	return Expr{node: &pg_query.Node{
		Node: &pg_query.Node_AConst{
			AConst: &pg_query.A_Const{
				Val: &pg_query.A_Const_Ival{
					Ival: &pg_query.Integer{Ival: n},
				},
			},
		},
	}}
}

// StringLiteralValue reports the value when expr is a string constant.
func (e Expr) StringLiteralValue() (string, bool) {
	if e.node == nil {
		return "", false
	}
	c := e.node.GetAConst()
	if c == nil || c.Isnull {
		return "", false
	}
	if sval := c.GetSval(); sval != nil {
		return sval.Sval, true
	}
	return "", false
}

// IsNull reports whether expr is the SQL NULL constant.
func (e Expr) IsNull() bool {
	if e.node == nil {
		return false
	}
	c := e.node.GetAConst()
	return c != nil && c.Isnull
}

// WriteWrapFunc replaces the expression assigned to a matched write-target column.
type WriteWrapFunc func(column Name, expr Expr) (Expr, error)

// WrapWriteExpressions rewrites UPDATE SET and INSERT VALUES/SELECT expressions
// for columns where match returns true. match sees the write-target table/column
// names as written in the query (before any Replace mapping).
func (q *Query) WrapWriteExpressions(match func(Name) bool, wrap WriteWrapFunc) error {
	if match == nil {
		return fmt.Errorf("sqltransform: WrapWriteExpressions match is nil")
	}
	if wrap == nil {
		return fmt.Errorf("sqltransform: WrapWriteExpressions wrap is nil")
	}
	return wrapwrite.Apply(q.tree,
		func(col extract.Name) bool {
			return match(Name(col))
		},
		func(col extract.Name, expr *pg_query.Node) (*pg_query.Node, error) {
			out, err := wrap(Name(col), Expr{node: expr})
			if err != nil {
				return nil, err
			}
			if out.node == nil {
				return nil, fmt.Errorf("sqltransform: wrap returned a nil expression for %s", Name(col).String())
			}
			return out.node, nil
		},
	)
}
