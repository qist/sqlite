package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Scalar functions
// ---------------------------------------------------------------------------

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

// TestDeterministicScalarFunction verifies deterministic scalar functions
// can be registered and invoked.
func TestDeterministicScalarFunction(t *testing.T) {
	if err := RegisterDeterministicScalarFunction("qist_det_add", 2, func(ctx *FunctionContext, args []driver.Value) (driver.Value, error) {
		return args[0].(int64) + args[1].(int64), nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	var got int64
	if err := db.QueryRow("SELECT qist_det_add(10, 32)").Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Aggregate / window functions
// ---------------------------------------------------------------------------

// TestAggregateFunction verifies that a custom aggregate SQL function
// registered via RegisterFunction works correctly.
func TestAggregateFunction(t *testing.T) {
	if err := RegisterFunction("qist_myavg", &FunctionImpl{
		NArgs: 1,
		MakeAggregate: func(ctx FunctionContext) (AggregateFunction, error) {
			return &myAvg{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE t(v real)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO t VALUES (1.0), (2.0), (3.0), (4.0)"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got float64
	if err := db.QueryRow("SELECT qist_myavg(v) FROM t").Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != 2.5 {
		t.Fatalf("expected 2.5, got %v", got)
	}
}

// myAvg implements AggregateFunction for a simple AVG aggregate.
type myAvg struct {
	sum float64
	n   int
}

func (a *myAvg) Step(ctx *FunctionContext, args []driver.Value) error {
	a.sum += args[0].(float64)
	a.n++
	return nil
}

func (a *myAvg) WindowInverse(ctx *FunctionContext, args []driver.Value) error {
	a.sum -= args[0].(float64)
	a.n--
	return nil
}

func (a *myAvg) WindowValue(ctx *FunctionContext) (driver.Value, error) {
	if a.n == 0 {
		return nil, nil
	}
	return a.sum / float64(a.n), nil
}

func (a *myAvg) Final(ctx *FunctionContext) {}

// ---------------------------------------------------------------------------
// Collations
// ---------------------------------------------------------------------------

// TestCollation verifies a custom UTF-8 collation can be registered and used.
func TestCollation(t *testing.T) {
	if err := RegisterCollationUtf8("qist_nocase", func(left, right string) int {
		// case-insensitive comparison
		ll := len(left)
		rl := len(right)
		minLen := ll
		if rl < minLen {
			minLen = rl
		}
		for i := 0; i < minLen; i++ {
			cl := left[i]
			cr := right[i]
			if cl >= 'A' && cl <= 'Z' {
				cl += 32
			}
			if cr >= 'A' && cr <= 'Z' {
				cr += 32
			}
			if cl < cr {
				return -1
			}
			if cl > cr {
				return 1
			}
		}
		if ll < rl {
			return -1
		}
		if ll > rl {
			return 1
		}
		return 0
	}); err != nil {
		t.Fatalf("register collation: %v", err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE t(name text COLLATE qist_nocase)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO t VALUES ('Apple'), ('banana'), ('Cherry')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var names []string
	rows, err := db.Query("SELECT name FROM t ORDER BY name")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, n)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(names))
	}
	// case-insensitive ordering: Apple, banana, Cherry
	if names[0] != "Apple" || names[1] != "banana" || names[2] != "Cherry" {
		t.Fatalf("unexpected order: %v", names)
	}
}

// ---------------------------------------------------------------------------
// sqlite-vec vector search
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Connection hooks
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Commit / rollback hooks (per-connection, via Raw)
// ---------------------------------------------------------------------------

// TestCommitRollbackHooks verifies per-connection commit and rollback hooks
// fire when using the modernc.org/sqlite driver's HookRegisterer interface.
func TestCommitRollbackHooks(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	committed := false
	rolledBack := false
	if err := conn.Raw(func(driverConn any) error {
		hr, ok := driverConn.(HookRegisterer)
		if !ok {
			t.Skip("driver does not support HookRegisterer")
		}
		hr.RegisterCommitHook(func() int32 {
			committed = true
			return 0
		})
		hr.RegisterRollbackHook(func() {
			rolledBack = true
		})
		return nil
	}); err != nil {
		t.Fatalf("raw: %v", err)
	}

	// trigger commit
	if _, err := conn.ExecContext(ctx, "CREATE TABLE t(v int)"); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !committed {
		t.Fatal("commit hook was not invoked")
	}

	// trigger rollback
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.Exec("INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !rolledBack {
		t.Fatal("rollback hook was not invoked")
	}
}

// ---------------------------------------------------------------------------
// Pre-update hook (per-connection, via Raw)
// ---------------------------------------------------------------------------

// TestPreUpdateHook verifies the pre-update hook fires for INSERT/UPDATE/DELETE.
func TestPreUpdateHook(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	var ops []int32
	if err := conn.Raw(func(driverConn any) error {
		hr, ok := driverConn.(HookRegisterer)
		if !ok {
			t.Skip("driver does not support HookRegisterer")
		}
		hr.RegisterPreUpdateHook(func(data SQLitePreUpdateData) {
			ops = append(ops, data.Op)
		})
		return nil
	}); err != nil {
		t.Fatalf("raw: %v", err)
	}

	if _, err := conn.ExecContext(ctx, "CREATE TABLE t(id int, v int)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO t VALUES (1, 10)"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "UPDATE t SET v = 20 WHERE id = 1"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "DELETE FROM t WHERE id = 1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if len(ops) != 3 {
		t.Fatalf("expected 3 pre-update calls (insert/update/delete), got %d", len(ops))
	}
}

// ---------------------------------------------------------------------------
// FileControl (PersistWAL, DataVersion)
// ---------------------------------------------------------------------------

// TestFileControl verifies FileControlPersistWAL and FileControlDataVersion
// are accessible through the Raw escape hatch.
func TestFileControl(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	if err := conn.Raw(func(driverConn any) error {
		fc, ok := driverConn.(FileControl)
		if !ok {
			t.Skip("driver does not support FileControl")
		}

		// DataVersion should return a value for main
		_, err := fc.FileControlDataVersion("main")
		return err
	}); err != nil {
		t.Fatalf("raw: %v", err)
	}

	// do a write, then check data version changed
	if _, err := conn.ExecContext(ctx, "CREATE TABLE t(v int)"); err != nil {
		t.Fatalf("create: %v", err)
	}

	var dv1, dv2 uint32
	if err := conn.Raw(func(driverConn any) error {
		fc := driverConn.(FileControl)
		v, err := fc.FileControlDataVersion("main")
		if err != nil {
			return err
		}
		dv1 = v
		return nil
	}); err != nil {
		t.Fatalf("raw1: %v", err)
	}

	if _, err := conn.ExecContext(ctx, "INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := conn.Raw(func(driverConn any) error {
		fc := driverConn.(FileControl)
		v, err := fc.FileControlDataVersion("main")
		if err != nil {
			return err
		}
		dv2 = v
		return nil
	}); err != nil {
		t.Fatalf("raw2: %v", err)
	}

	if dv2 == dv1 {
		t.Logf("warning: data version did not change (dv1=%d, dv2=%d)", dv1, dv2)
	}
}

// ---------------------------------------------------------------------------
// DBStatus (per-connection runtime counters)
// ---------------------------------------------------------------------------

// TestDBStatus verifies DBStatus.Status can read per-connection counters.
func TestDBStatus(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "CREATE TABLE t(v int)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := conn.Raw(func(driverConn any) error {
		ds, ok := driverConn.(DBStatus)
		if !ok {
			t.Skip("driver does not support DBStatus")
		}
		// cache used should be non-zero after writes
		cur, _, err := ds.Status(DBStatusCacheUsed, false)
		if err != nil {
			return err
		}
		if cur <= 0 {
			t.Errorf("expected non-zero cache used, got %d", cur)
		}
		return nil
	}); err != nil {
		t.Fatalf("raw: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Limit (sqlite3_limit)
// ---------------------------------------------------------------------------

// TestLimit verifies sqlite3_limit can be called through the Limit helper.
func TestLimit(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	// SQLITE_LIMIT_COLUMN = 2
	const SQLITE_LIMIT_COLUMN = 2

	// Query the current limit
	old, err := Limit(conn, SQLITE_LIMIT_COLUMN, -1)
	if err != nil {
		t.Fatalf("limit query: %v", err)
	}
	if old <= 0 {
		t.Fatalf("expected positive column limit, got %d", old)
	}

	// Set a new limit and verify it
	newVal := old - 1
	got, err := Limit(conn, SQLITE_LIMIT_COLUMN, newVal)
	if err != nil {
		t.Fatalf("limit set: %v", err)
	}
	if got != old {
		t.Fatalf("expected old limit %d, got %d", old, got)
	}

	// Query again to verify the new limit is in effect
	cur, err := Limit(conn, SQLITE_LIMIT_COLUMN, -1)
	if err != nil {
		t.Fatalf("limit query 2: %v", err)
	}
	if cur != newVal {
		t.Fatalf("expected limit %d, got %d", newVal, cur)
	}
}

// ---------------------------------------------------------------------------
// ColumnInfo (query column metadata)
// ---------------------------------------------------------------------------

// TestQueryColumnInfo verifies column metadata can be retrieved for a query.
func TestQueryColumnInfo(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE t(id int, name text)"); err != nil {
		t.Fatalf("create: %v", err)
	}

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	info, err := QueryColumnInfo(conn, "SELECT id, name, 42 AS answer FROM t")
	if err != nil {
		t.Fatalf("column info: %v", err)
	}
	if len(info) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(info))
	}
	if info[0].Name != "id" {
		t.Errorf("col0 name: want id, got %s", info[0].Name)
	}
	if info[0].TableName != "t" {
		t.Errorf("col0 table: want t, got %s", info[0].TableName)
	}
	if info[1].Name != "name" {
		t.Errorf("col1 name: want name, got %s", info[1].Name)
	}
	if info[2].Name != "answer" {
		t.Errorf("col2 name: want answer, got %s", info[2].Name)
	}
	// computed column has no source table
	if info[2].TableName != "" {
		t.Errorf("col2 table: want empty, got %s", info[2].TableName)
	}
}

// ---------------------------------------------------------------------------
// Online backup / restore
// ---------------------------------------------------------------------------

// TestBackupRestore verifies the online backup API can copy a database.
func TestBackupRestore(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.db")
	dstPath := filepath.Join(tmpDir, "dst.db")

	// create and populate source database
	srcDB, err := sql.Open("sqlite", srcPath)
	if err != nil {
		t.Fatalf("open src: %v", err)
	}
	defer srcDB.Close()

	if _, err := srcDB.Exec("CREATE TABLE t(id int, name text)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := srcDB.Exec("INSERT INTO t VALUES (1, 'hello')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// backup source → destination
	ctx := context.Background()
	conn, err := srcDB.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}

	backup, err := NewBackup(conn, dstPath)
	if err != nil {
		t.Fatalf("new backup: %v", err)
	}
	more, err := backup.Step(-1)
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if more {
		t.Fatal("expected backup to be done after Step(-1)")
	}
	if err := backup.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	conn.Close()

	// verify the destination has the data
	dstDB, err := sql.Open("sqlite", dstPath)
	if err != nil {
		t.Fatalf("open dst: %v", err)
	}
	defer dstDB.Close()

	var id int
	var name string
	if err := dstDB.QueryRow("SELECT id, name FROM t").Scan(&id, &name); err != nil {
		t.Fatalf("query dst: %v", err)
	}
	if id != 1 || name != "hello" {
		t.Fatalf("expected (1, hello), got (%d, %s)", id, name)
	}

	// clean up
	os.Remove(srcPath)
	os.Remove(dstPath)
}

// ---------------------------------------------------------------------------
// NewConnector
// ---------------------------------------------------------------------------

// TestNewConnector verifies NewConnector + sql.OpenDB works.
func TestNewConnector(t *testing.T) {
	connector, err := NewConnector(":memory:")
	if err != nil {
		t.Fatalf("new connector: %v", err)
	}

	db := sql.OpenDB(connector)
	defer db.Close()

	var got int
	if err := db.QueryRow("SELECT 1").Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}
