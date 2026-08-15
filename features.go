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

// FunctionContext is the context user-defined SQL functions execute in.
type FunctionContext = msqlite.FunctionContext

// ConnectionHookFn is called after each new connection is opened. Use it to
// apply per-connection PRAGMAs (e.g. busy_timeout, journal_mode) uniformly.
type ConnectionHookFn = msqlite.ConnectionHookFn

// PageCache is the factory for per-database custom page caches. Supply an
// implementation via RegisterPageCache to override SQLite's default pcache2.
// It MUST be installed before the first connection is opened in the process.
type PageCache = msqlite.PageCache

// ---------------------------------------------------------------------------
// Registration helpers (process-global, apply to every new connection)
// ---------------------------------------------------------------------------

// RegisterScalarFunction registers a custom scalar SQL function (named
// zFuncName with nArg arguments; pass -1 for variadic). The function becomes
// available to every new connection opened afterwards.
func RegisterScalarFunction(zFuncName string, nArg int32, xFunc func(ctx *FunctionContext, args []driver.Value) (driver.Value, error)) error {
	return msqlite.RegisterScalarFunction(zFuncName, nArg, xFunc)
}

// RegisterDeterministicScalarFunction is like RegisterScalarFunction but marks
// the function as deterministic (same output for same inputs), allowing SQLite
// to invoke it during query planning and cache its results.
func RegisterDeterministicScalarFunction(zFuncName string, nArg int32, xFunc func(ctx *FunctionContext, args []driver.Value) (driver.Value, error)) error {
	return msqlite.RegisterDeterministicScalarFunction(zFuncName, nArg, xFunc)
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

// RegisterVirtualTable registers a pure-Go virtual table module (e.g. an FTS-like
// or computed-source table) with the given *sql.DB. The module is applied to
// connections opened afterwards. The Module interface comes from
// modernc.org/sqlite/vtab.
func RegisterVirtualTable(db *sql.DB, name string, m vtab.Module) error {
	return vtab.RegisterModule(db, name, m)
}
