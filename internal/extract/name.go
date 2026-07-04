package extract

// Name is the internal physical reference name (mirrors public sqltransform.Name).
type Name struct {
	Schema string
	Table  string
	Column string
}
