package sqltransform

import "fmt"

// Command is the top-level DML statement type.
type Command string

const (
	CommandSelect Command = "SELECT"
	CommandInsert Command = "INSERT"
	CommandUpdate Command = "UPDATE"
	CommandDelete Command = "DELETE"
)

// ErrUnsupportedStatement is returned when the parsed SQL is not a supported DML statement.
var ErrUnsupportedStatement = fmt.Errorf("sqltransform: unsupported statement type")
