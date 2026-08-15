package sqlite

import (
	"database/sql/driver"

	gosqlite "github.com/glebarez/go-sqlite"
)

// FunctionContext is the context user-defined SQL functions execute in. It is
// re-exported here so callers can register custom functions without importing
// github.com/glebarez/go-sqlite directly.
type FunctionContext = gosqlite.FunctionContext

// RegisterScalarFunction registers a custom scalar SQL function (named
// zFuncName with nArg arguments; pass -1 for variadic). The function becomes
// available to every new connection opened afterwards through this driver.
//
// This forwards to github.com/glebarez/go-sqlite, which in turn exposes the
// underlying modernc.org/sqlite custom-function capability.
func RegisterScalarFunction(zFuncName string, nArg int32, xFunc func(ctx *FunctionContext, args []driver.Value) (driver.Value, error)) error {
	return gosqlite.RegisterScalarFunction(zFuncName, nArg, xFunc)
}

// RegisterDeterministicScalarFunction is like RegisterScalarFunction but marks
// the function as deterministic (same output for same inputs), allowing SQLite
// to invoke it during query planning and cache its results.
func RegisterDeterministicScalarFunction(zFuncName string, nArg int32, xFunc func(ctx *FunctionContext, args []driver.Value) (driver.Value, error)) error {
	return gosqlite.RegisterDeterministicScalarFunction(zFuncName, nArg, xFunc)
}
