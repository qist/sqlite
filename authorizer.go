package sqlite

import (
	msqlite "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// SQLite statement authorizer (modernc.org/sqlite v1.61.0+)
// ---------------------------------------------------------------------------

// AuthorizerActionCode identifies an operation presented to SQLite's
// connection authorizer while a statement is compiled.
type AuthorizerActionCode = msqlite.AuthorizerActionCode

// AuthorizerReturnCode is a result returned by an AuthorizerFn.
type AuthorizerReturnCode = msqlite.AuthorizerReturnCode

// AuthorizerFn decides whether SQLite may compile an operation. The arguments
// have the same meaning as the third through sixth arguments of SQLite's
// authorizer callback. A callback must not panic or modify the SQLite
// connection that invoked it, including by preparing or stepping SQL on that
// connection.
//
// SQLite NULL callback arguments are passed as empty strings. SQLite invokes
// the callback during statement preparation and may invoke it again when a
// statement is automatically reprepared. BEGIN, COMMIT and ROLLBACK are
// presented as AuthTransaction, so denying that action can make BeginTx or
// transaction completion fail.
type AuthorizerFn = msqlite.AuthorizerFn

// AuthorizerRegisterer exposes the authorizer of a physical SQLite connection.
// Reach it through (*sql.Conn).Raw, or install a policy on every pooled
// connection from RegisterConnectionHook (using ExecQuerierContext):
//
//	sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
//	    r, ok := conn.(sqlite.AuthorizerRegisterer)
//	    if !ok {
//	        return fmt.Errorf("driver does not support an authorizer")
//	    }
//	    return r.RegisterAuthorizer(fn)
//	})
//
// RegisterAuthorizer replaces any authorizer already installed on the
// connection; passing nil removes it. The policy remains on that physical
// connection after the *sql.Conn is returned to the pool, so prefer the
// connection-hook pattern when every pooled connection must share one policy.
// Registering, replacing or removing an authorizer expires prepared statements
// on that connection; SQLite prepares them again on next use.
type AuthorizerRegisterer = msqlite.AuthorizerRegisterer

// Auth* are the actions presented to an AuthorizerFn.
const (
	AuthCreateIndex       = msqlite.AuthCreateIndex
	AuthCreateTable       = msqlite.AuthCreateTable
	AuthCreateTempIndex   = msqlite.AuthCreateTempIndex
	AuthCreateTempTable   = msqlite.AuthCreateTempTable
	AuthCreateTempTrigger = msqlite.AuthCreateTempTrigger
	AuthCreateTempView    = msqlite.AuthCreateTempView
	AuthCreateTrigger     = msqlite.AuthCreateTrigger
	AuthCreateView        = msqlite.AuthCreateView
	AuthDelete            = msqlite.AuthDelete
	AuthDropIndex         = msqlite.AuthDropIndex
	AuthDropTable         = msqlite.AuthDropTable
	AuthDropTempIndex     = msqlite.AuthDropTempIndex
	AuthDropTempTable     = msqlite.AuthDropTempTable
	AuthDropTempTrigger   = msqlite.AuthDropTempTrigger
	AuthDropTempView      = msqlite.AuthDropTempView
	AuthDropTrigger       = msqlite.AuthDropTrigger
	AuthDropView          = msqlite.AuthDropView
	AuthInsert            = msqlite.AuthInsert
	AuthPragma            = msqlite.AuthPragma
	AuthRead              = msqlite.AuthRead
	AuthSelect            = msqlite.AuthSelect
	AuthTransaction       = msqlite.AuthTransaction
	AuthUpdate            = msqlite.AuthUpdate
	AuthAttach            = msqlite.AuthAttach
	AuthDetach            = msqlite.AuthDetach
	AuthAlterTable        = msqlite.AuthAlterTable
	AuthReindex           = msqlite.AuthReindex
	AuthAnalyze           = msqlite.AuthAnalyze
	AuthCreateVTable      = msqlite.AuthCreateVTable
	AuthDropVTable        = msqlite.AuthDropVTable
	AuthFunction          = msqlite.AuthFunction
	AuthSavepoint         = msqlite.AuthSavepoint
	AuthRecursive         = msqlite.AuthRecursive
)

// Authorizer return codes control how SQLite handles an operation.
const (
	AuthorizerOK     = msqlite.AuthorizerOK
	AuthorizerDeny   = msqlite.AuthorizerDeny
	AuthorizerIgnore = msqlite.AuthorizerIgnore
)
