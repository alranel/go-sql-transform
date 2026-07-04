package sqltransform

import (
	"fmt"

	"github.com/alranel/go-sql-transform/internal/replace"
)

// Replace renames a table or column throughout the query AST.
//
// For table replacement, set from.Table (and optionally from.Schema) with Column empty.
// For column replacement, set from.Column; set from.Table to disambiguate by physical table.
func (q *Query) Replace(from, to Name) error {
	if err := from.Validate(true); err != nil {
		return err
	}
	if err := to.Validate(true); err != nil {
		return err
	}
	if from.Column == "" {
		if to.Table == "" {
			return fmt.Errorf("sqltransform: to.Table required for table replacement")
		}
	} else if to.Column == "" {
		return fmt.Errorf("sqltransform: to.Column required for column replacement")
	}
	replace.Apply(q.tree, replace.Name(from), replace.Name(to))
	return nil
}
