package sqltransform

import (
	"github.com/alranel/go-sql-transform/internal/expandstar"
	"github.com/alranel/go-sql-transform/internal/extract"
)

// ExpandStarFunc returns explicit columns to substitute for table.* in SELECT projections.
type ExpandStarFunc func(table Name) ([]Name, error)

// ExpandStar replaces SELECT * / table.* in projection and RETURNING lists with explicit columns.
func (q *Query) ExpandStar(expand ExpandStarFunc) error {
	return expandstar.Apply(q.tree, func(table extract.Name) ([]extract.Name, error) {
		cols, err := expand(Name(table))
		if err != nil {
			return nil, err
		}
		out := make([]extract.Name, len(cols))
		for i, c := range cols {
			out[i] = extract.Name(c)
		}
		return out, nil
	})
}
