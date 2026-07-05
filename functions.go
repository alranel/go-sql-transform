package sqltransform

import "github.com/alranel/go-sql-transform/internal/extract"

// FuncCall is a function invocation in the query.
type FuncCall struct {
	Schema string // optional catalog/schema prefix
	Name   string // function name (last component)
}

// FunctionCalls returns deduplicated function invocations in the query.
func (q *Query) FunctionCalls() []FuncCall {
	calls := extract.FuncCalls(q.tree)
	out := make([]FuncCall, len(calls))
	for i, c := range calls {
		out[i] = FuncCall{Schema: c.Schema, Name: c.Name}
	}
	return out
}
