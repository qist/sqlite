package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	msqlite "modernc.org/sqlite"
)

// ExecQuerierContext is the connection handle passed to a connection hook.
type ExecQuerierContext = msqlite.ExecQuerierContext

// TestCustomScalarFunction verifies a user-registered scalar SQL function is
// callable through the driver (modernc.org/sqlite capability).
func TestCustomScalarFunction(t *testing.T) {
	if err := RegisterScalarFunction("qist_double", 1, func(ctx *FunctionContext, args []driver.Value) (driver.Value, error) {
		return args[0].(int64) * 2, nil
	}); err != nil {
		t.Fatalf("register function: %v", err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	var got int64
	if err := db.QueryRow("SELECT qist_double(21)").Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

// TestVecExtension verifies the sqlite-vec vector-search extension is wired in
// through the blank import of modernc.org/sqlite/vec.
func TestVecExtension(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err = db.Exec("CREATE VIRTUAL TABLE vec_items USING vec0(embedding float[4])"); err != nil {
		t.Fatalf("create vec0 table: %v", err)
	}
	if _, err = db.Exec("INSERT INTO vec_items(rowid, embedding) VALUES (1, '[1,2,3,4]')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var dist float64
	if err = db.QueryRow("SELECT distance FROM vec_items WHERE embedding MATCH '[1,2,3,4]' ORDER BY distance LIMIT 1").Scan(&dist); err != nil {
		t.Fatalf("vec query: %v", err)
	}
	if dist != 0 {
		t.Fatalf("expected distance 0 for identical vector, got %v", dist)
	}
}

// TestConnectionHook verifies a per-connection hook (e.g. to set PRAGMAs) runs.
func TestConnectionHook(t *testing.T) {
	called := false
	RegisterConnectionHook(func(conn ExecQuerierContext, dsn string) error {
		called = true
		return nil
	})

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if !called {
		t.Fatal("connection hook was not invoked")
	}
}
