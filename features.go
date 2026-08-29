package sqlite

import (
	"database/sql"
	"database/sql/driver"

	msqlite "modernc.org/sqlite"
	"modernc.org/sqlite/vtab"

	// Blank import wires the sqlite-vec vector-search extension
	// (modernc.org/sqlite/vec) into every connection opened via this driver.
	// It installs the vec0 virtual table module and the vec_* SQL functions
	// through sqlite3_auto_extension, so no further API call is required.
	// See https://github.com/asg017/sqlite-vec for usage.
	_ "modernc.org/sqlite/vec"
)

// ---------------------------------------------------------------------------
// Type aliases (re-export modernc.org/sqlite types so users don't need to
// import the upstream package directly)
// ---------------------------------------------------------------------------

// FunctionContext is the context user-defined SQL functions execute in.
type FunctionContext = msqlite.FunctionContext

// FunctionImpl describes an application-defined SQL function. If Scalar is set
// it is treated as a scalar function; otherwise it is treated as an aggregate
// or window function using MakeAggregate.
type FunctionImpl = msqlite.FunctionImpl

// AggregateFunction is an invocation of an aggregate or window function.
// Implement this interface (returned by FunctionImpl.MakeAggregate) to
// register custom aggregate/window SQL functions.
type AggregateFunction = msqlite.AggregateFunction

// ConnectionHookFn is called after each new connection is opened. Use it to
// apply per-connection PRAGMAs (e.g. busy_timeout, journal_mode) uniformly.
type ConnectionHookFn = msqlite.ConnectionHookFn

// ExecQuerierContext is the connection handle passed to a connection hook.
type ExecQuerierContext = msqlite.ExecQuerierContext

// HookRegisterer is the interface a *sql.Conn's underlying driver.Conn
// satisfies when the modernc.org/sqlite driver is in use. It allows
// registering per-connection pre-update, commit and rollback hooks.
// Access it through (*sql.Conn).Raw.
type HookRegisterer = msqlite.HookRegisterer

// PageCache is the factory for per-database custom page caches. Supply an
// implementation via RegisterPageCache to override SQLite's default pcache2.
// It MUST be installed before the first connection is opened in the process.
type PageCache = msqlite.PageCache

// Backup manages an online (hot) backup or restore between two databases.
// Obtain one via NewBackup or NewRestore.
type Backup = msqlite.Backup

// FileControl exposes sqlite3_file_control operations. Reach it through
// (*sql.Conn).Raw, the same way as DBStatus.
type FileControl = msqlite.FileControl

// DBStatus exposes per-connection runtime counters (cache hit/miss/write,
// lookaside usage, deferred FKs, etc.) via sqlite3_db_status.
type DBStatus = msqlite.DBStatus

// DBStatusOp identifies a per-connection runtime counter readable through
// DBStatus.Status.
type DBStatusOp = msqlite.DBStatusOp

// PreUpdateHookFn is called before each INSERT/UPDATE/DELETE on a connection
// that has registered a pre-update hook.
type PreUpdateHookFn = msqlite.PreUpdateHookFn

// CommitHookFn is called when a transaction commits. Return non-zero to
// convert the commit into a rollback.
type CommitHookFn = msqlite.CommitHookFn

// RollbackHookFn is called when a transaction rolls back.
type RollbackHookFn = msqlite.RollbackHookFn

// SQLitePreUpdateData carries row change details to a PreUpdateHookFn.
type SQLitePreUpdateData = msqlite.SQLitePreUpdateData

// ColumnInfo describes one output column of a prepared SQL statement.
type ColumnInfo = msqlite.ColumnInfo

// Driver is a modernc.org/sqlite driver instance. Since upstream v1.57.0 a
// caller-constructed *Driver can register its own SQL functions, collations
// and virtual table modules without affecting the process-global "sqlite"
// driver. The zero value is ready to use; construct one with NewDriver and
// register it under a unique name via sql.Register (or OpenDriver), so that
// its functions/modules apply only to connections opened through it.
//
//	mine := sqlite.NewDriver()
//	mine.RegisterScalarFunction("hello", 0, func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
//	    return "world", nil
//	})
//	db, err := sqlite.OpenDriver(mine, "sqlite-mine", "test.db")
//	// sql.Open("sqlite", ...) connections do NOT see hello().
type Driver = msqlite.Driver

// NewDriver returns a new *Driver ready to register instance-local SQL
// functions, collations and virtual table modules. Registrations on it are
// isolated from the process-global driver and from other Driver instances.
func NewDriver() *Driver {
	return &msqlite.Driver{}
}

// OpenDriver registers d under the given driver name (if it is not already
// registered) and opens a database on dsn through it. Connections made via the
// returned *sql.DB only see functions/collations/modules registered on d, not
// those registered on the process-global driver.
//
// Registering under an already-used name is a no-op here (the existing
// registration wins), matching database/sql's global registry semantics;
// choose a name unique to your application.
func OpenDriver(d *Driver, name, dsn string) (*sql.DB, error) {
	registered := false
	for _, n := range sql.Drivers() {
		if n == name {
			registered = true
			break
		}
	}
	if !registered {
		sql.Register(name, d)
	}
	return sql.Open(name, dsn)
}

// BackupConn is the interface satisfied by a *sql.Conn's underlying
// driver.Conn when using the modernc.org/sqlite driver. It exposes
// online backup/restore and column introspection. Access it through
// (*sql.Conn).Raw:
//
//	err := sqlConn.Raw(func(dc any) error {
//	    bc := dc.(sqlite.BackupConn)
//	    backup, err := bc.NewBackup("file:backup.db")
//	    // ...
//	})
type BackupConn interface {
	// NewBackup creates an online backup of the current database to the
	// database pointed by dstUri.
	NewBackup(dstUri string) (*Backup, error)
	// NewRestore creates an online restore from the database pointed by
	// srcUri into the current database.
	NewRestore(srcUri string) (*Backup, error)
	// ColumnInfo returns metadata about the output columns of a query.
	ColumnInfo(query string) ([]ColumnInfo, error)
}

// ---------------------------------------------------------------------------
// Constants (re-export DBStatus operations)
// ---------------------------------------------------------------------------

// DBStatus* constants are the operations accepted by DBStatus.Status.
// See https://www.sqlite.org/c3ref/c_dbstatus_options.html for semantics.
const (
	DBStatusLookasideUsed     = msqlite.DBStatusLookasideUsed
	DBStatusCacheUsed         = msqlite.DBStatusCacheUsed
	DBStatusSchemaUsed        = msqlite.DBStatusSchemaUsed
	DBStatusStmtUsed          = msqlite.DBStatusStmtUsed
	DBStatusLookasideHit      = msqlite.DBStatusLookasideHit
	DBStatusLookasideMissSize = msqlite.DBStatusLookasideMissSize
	DBStatusLookasideMissFull = msqlite.DBStatusLookasideMissFull
	DBStatusCacheHit          = msqlite.DBStatusCacheHit
	DBStatusCacheMiss         = msqlite.DBStatusCacheMiss
	DBStatusCacheWrite        = msqlite.DBStatusCacheWrite
	DBStatusDeferredFKs       = msqlite.DBStatusDeferredFKs
	DBStatusCacheUsedShared   = msqlite.DBStatusCacheUsedShared
	DBStatusCacheSpill        = msqlite.DBStatusCacheSpill
	DBStatusTempbufSpill      = msqlite.DBStatusTempbufSpill
)

// ---------------------------------------------------------------------------
// Registration helpers (process-global, apply to every new connection)
// ---------------------------------------------------------------------------

// RegisterScalarFunction registers a custom scalar SQL function (named
// zFuncName with nArg arguments; pass -1 for variadic). The function becomes
// available to every new connection opened afterwards.
func RegisterScalarFunction(zFuncName string, nArg int32, xFunc func(ctx *FunctionContext, args []driver.Value) (driver.Value, error)) error {
	return msqlite.RegisterScalarFunction(zFuncName, nArg, xFunc)
}

// MustRegisterScalarFunction is like RegisterScalarFunction but panics on error.
func MustRegisterScalarFunction(zFuncName string, nArg int32, xFunc func(ctx *FunctionContext, args []driver.Value) (driver.Value, error)) {
	msqlite.MustRegisterScalarFunction(zFuncName, nArg, xFunc)
}

// RegisterDeterministicScalarFunction is like RegisterScalarFunction but marks
// the function as deterministic (same output for same inputs), allowing SQLite
// to invoke it during query planning and cache its results.
func RegisterDeterministicScalarFunction(zFuncName string, nArg int32, xFunc func(ctx *FunctionContext, args []driver.Value) (driver.Value, error)) error {
	return msqlite.RegisterDeterministicScalarFunction(zFuncName, nArg, xFunc)
}

// MustRegisterDeterministicScalarFunction is like
// RegisterDeterministicScalarFunction but panics on error.
func MustRegisterDeterministicScalarFunction(zFuncName string, nArg int32, xFunc func(ctx *FunctionContext, args []driver.Value) (driver.Value, error)) {
	msqlite.MustRegisterDeterministicScalarFunction(zFuncName, nArg, xFunc)
}

// RegisterFunction registers a custom SQL function — scalar, aggregate, or
// window — described by impl. When impl.Scalar is set the function is scalar;
// otherwise impl.MakeAggregate must be provided for an aggregate/window function.
// The function becomes available to every new connection opened afterwards.
//
// For aggregate/window functions, implement the AggregateFunction interface:
//
//	type myAvg struct{ sum float64; n int }
//	func (a *myAvg) Step(ctx *sqlite.FunctionContext, args []driver.Value) error {
//	    a.sum += args[0].(float64); a.n++
//	    return nil
//	}
//	func (a *myAvg) WindowInverse(ctx *sqlite.FunctionContext, args []driver.Value) error {
//	    a.sum -= args[0].(float64); a.n--
//	    return nil
//	}
//	func (a *myAvg) WindowValue(ctx *sqlite.FunctionContext) (driver.Value, error) {
//	    return a.sum / float64(a.n), nil
//	}
//	func (a *myAvg) Final(ctx *sqlite.FunctionContext) {}
func RegisterFunction(zFuncName string, impl *FunctionImpl) error {
	return msqlite.RegisterFunction(zFuncName, impl)
}

// MustRegisterFunction is like RegisterFunction but panics on error.
func MustRegisterFunction(zFuncName string, impl *FunctionImpl) {
	msqlite.MustRegisterFunction(zFuncName, impl)
}

// RegisterCollationUtf8 makes a Go function available as a UTF-8 collation
// named zName. The impl function receives two strings (left, right) and must
// return 0 if equal, negative if left < right, positive if left > right.
// The collation becomes available to all new connections opened afterwards.
func RegisterCollationUtf8(zName string, impl func(left, right string) int) error {
	return msqlite.RegisterCollationUtf8(zName, impl)
}

// MustRegisterCollationUtf8 is like RegisterCollationUtf8 but panics on error.
func MustRegisterCollationUtf8(zName string, impl func(left, right string) int) {
	msqlite.MustRegisterCollationUtf8(zName, impl)
}

// RegisterConnectionHook registers a function invoked after each connection is
// opened. Multiple hooks can be registered; they run in registration order.
func RegisterConnectionHook(fn ConnectionHookFn) {
	msqlite.RegisterConnectionHook(fn)
}

// RegisterPageCache installs a process-global custom page cache via
// SQLITE_CONFIG_PCACHE2. It returns an error if called after a connection has
// already been opened.
func RegisterPageCache(m PageCache) error {
	return msqlite.RegisterPageCache(m)
}

// MustRegisterPageCache is like RegisterPageCache but panics on error.
func MustRegisterPageCache(m PageCache) {
	msqlite.MustRegisterPageCache(m)
}

// RegisterVirtualTable registers a pure-Go virtual table module (e.g. an FTS-like
// or computed-source table) with the given *sql.DB. The module is applied to
// connections opened afterwards. The Module interface comes from
// modernc.org/sqlite/vtab.
//
// Since upstream v1.57.0, if db was opened on a caller-constructed Driver (via
// OpenDriver), the module is registered on that Driver alone and stays
// isolated from the process-global driver; a nil db targets the global
// "sqlite" driver.
func RegisterVirtualTable(db *sql.DB, name string, m vtab.Module) error {
	return vtab.RegisterModule(db, name, m)
}

// NewConnector returns a driver.Connector that opens connections using the
// registered driver. Use it with sql.OpenDB when you need to interpose on
// physical connections (tracing, metrics, etc.) without a process-global
// sql.Register.
func NewConnector(dsn string) (driver.Connector, error) {
	return msqlite.NewConnector(dsn)
}

// Limit calls sqlite3_limit on the given *sql.Conn, setting or querying
// runtime limits such as SQLITE_LIMIT_SQL_LENGTH, SQLITE_LIMIT_COLUMN, etc.
// See https://www.sqlite.org/c3ref/limit.html for details.
func Limit(c *sql.Conn, id int, newVal int) (r int, err error) {
	return msqlite.Limit(c, id, newVal)
}

// NewBackup creates an online backup of the database on the given *sql.Conn
// to the database pointed by dstUri. The returned Backup must be driven with
// Step and released with Finish (or Commit).
//
//	sqlConn, _ := db.Conn(ctx)
//	backup, err := sqlite.NewBackup(sqlConn, "file:backup.db")
//	if err != nil { return err }
//	for more, _ := backup.Step(100); more; more, _ = backup.Step(100) {}
//	backup.Finish()
func NewBackup(c *sql.Conn, dstUri string) (*Backup, error) {
	var backup *Backup
	err := c.Raw(func(driverConn any) error {
		bc, ok := driverConn.(BackupConn)
		if !ok {
			return driver.ErrSkip
		}
		var err error
		backup, err = bc.NewBackup(dstUri)
		return err
	})
	return backup, err
}

// NewRestore creates an online restore from the database pointed by srcUri
// into the database on the given *sql.Conn.
func NewRestore(c *sql.Conn, srcUri string) (*Backup, error) {
	var backup *Backup
	err := c.Raw(func(driverConn any) error {
		bc, ok := driverConn.(BackupConn)
		if !ok {
			return driver.ErrSkip
		}
		var err error
		backup, err = bc.NewRestore(srcUri)
		return err
	})
	return backup, err
}

// QueryColumnInfo returns metadata about the output columns of a query — name,
// declared type, source database/table/column — by preparing the statement
// on the given *sql.Conn and introspecting it.
func QueryColumnInfo(c *sql.Conn, query string) ([]ColumnInfo, error) {
	var info []ColumnInfo
	err := c.Raw(func(driverConn any) error {
		bc, ok := driverConn.(BackupConn)
		if !ok {
			return driver.ErrSkip
		}
		var err error
		info, err = bc.ColumnInfo(query)
		return err
	})
	return info, err
}
