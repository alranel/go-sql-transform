package sqltransform_test

import (
	"testing"

	"github.com/alranel/go-sql-transform"
)

func TestUnqualifiedColumnReferences(t *testing.T) {
	q, err := sqltransform.Parse("SELECT author FROM book")
	if err != nil {
		t.Fatal(err)
	}
	got := q.UnqualifiedColumnReferences()
	if len(got) != 1 || got[0] != "author" {
		t.Fatalf("got %v, want [author]", got)
	}
}

func TestUnqualifiedColumnReferences_qualified(t *testing.T) {
	q, err := sqltransform.Parse("SELECT book.author FROM book")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.UnqualifiedColumnReferences()) != 0 {
		t.Fatalf("expected no unqualified refs, got %v", q.UnqualifiedColumnReferences())
	}
}

func TestUnqualifiedColumnReferences_updateSetNameExcluded(t *testing.T) {
	q, err := sqltransform.Parse("UPDATE book SET author = 'x' WHERE book.id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.UnqualifiedColumnReferences()) != 0 {
		t.Fatalf("SET target name should not count, got %v", q.UnqualifiedColumnReferences())
	}
}
