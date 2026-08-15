# Pure-Go SQLite driver for GORM
Pure-go (without cgo) implementation of SQLite driver for [GORM](https://gorm.io/)<br><br>
This driver has SQLite embedded, you don't need to install one separately.

This is an independent, pure-Go SQLite driver for GORM. The underlying engine is [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go, zero cgo), and it exposes additional capabilities such as custom SQL functions, connection hooks, a pluggable page cache, virtual tables (vtab), and the sqlite-vec vector extension.

# Usage

```go
import (
  "github.com/qist/sqlite"
  "gorm.io/gorm"
)

db, err := gorm.Open(sqlite.Open("sqlite.db"), &gorm.Config{})
```

### In-memory DB example
```go
db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
```

### Foreign-key constraint activation
Foreign-key constraint is disabled by default in SQLite. To activate it, use connection URL parameter:
```go
db, err := gorm.Open(sqlite.Open(":memory:?_pragma=foreign_keys(1)"), &gorm.Config{})
```
More info: [https://www.sqlite.org/foreignkeys.html](https://www.sqlite.org/foreignkeys.html)

# FAQ
## How is this better than standard GORM SQLite driver?
The [standard GORM driver for SQLite](https://github.com/go-gorm/sqlite) has one major drawback: it is based on a [Go-bindings of SQLite C-source](https://github.com/mattn/go-sqlite3) (this is called [cgo](https://go.dev/blog/cgo)). This fact imposes following restrictions on Go developers:
- to build and run your code, you will need a C compiler installed on a machine
- SQLite has many features that need to be enabled at compile time (e.g. [json support](https://www.sqlite.org/json1.html)). If you plan to use those, you will have to include proper build tags for every ```go``` command to work properly (```go run```, ```go test```, etc.).
- Because of C-compiler requirement, you can't build your Go code inside tiny stripped containers like (golang-alpine)
- Building on GCP is not possible because Google Cloud Platform does not allow gcc to be executed.

**Instead**, this driver is based on pure-Go implementation of SQLite (https://gitlab.com/cznic/sqlite), which is basically an original SQLite C-source AST, translated into Go! So, you may be sure you're using the original SQLite implementation under the hood.

## Is this tested good ?
Yes, The CI pipeline of this driver employs [whole test base](https://github.com/go-gorm/gorm/tree/master/tests) of GORM, which includes more than **12k** tests. Testing is run against latest major releases of Go:
- 1.25
- 1.26

In following environments:
- Linux
- Windows
- MacOS

## Is it fast?
Well, it's slower than CGo implementation, but not terribly. See the [bechmark of underlying pure-Go driver vs CGo implementation](https://github.com/glebarez/go-sqlite/tree/master/benchmark).

## Included features
-  JSON1 (https://www.sqlite.org/json1.html)
-  Math functions (https://www.sqlite.org/lang_mathfunc.html)
-  Generated (computed) columns via the `generated` tag, e.g.:
   ```go
   type Product struct {
     gorm.Model
     Price    float64
     Quantity float64
     Total    float64 `gorm:"generated:price * quantity"`
   }
   ```
-  Reliable table migrations: constraints (UNIQUE / CHECK / PRIMARY KEY) and
   generated columns are preserved when GORM rebuilds a table.
-  Extra SQLite capabilities exposed via package-level helpers (no GORM API
   changes required), backed by modernc.org/sqlite:
   - **Custom SQL functions** — `sqlite.RegisterScalarFunction` /
     `sqlite.RegisterDeterministicScalarFunction` /
     `sqlite.RegisterFunction` (scalar, aggregate, and window functions)
   - **Custom collations** — `sqlite.RegisterCollationUtf8`
   - **Per-connection hooks** (e.g. to set `PRAGMA`s uniformly) —
     `sqlite.RegisterConnectionHook`
   - **Commit / rollback / pre-update hooks** — per-connection, via
     `sqlite.HookRegisterer` through `(*sql.Conn).Raw`
   - **Custom page cache** — `sqlite.RegisterPageCache` (must be called before
     the first connection)
   - **Virtual tables** — `sqlite.RegisterVirtualTable` (pure-Go `vtab` modules)
   - **sqlite-vec vector search** — available out of the box via the `vec0`
     virtual table and `vec_*` SQL functions (no extra setup)
   - **Online backup / restore** — `sqlite.NewBackup` / `sqlite.NewRestore`
     for hot backups between databases
   - **Per-connection runtime stats** — `sqlite.DBStatus` interface (cache
     hit/miss/write, lookaside, deferred FKs, etc.)
   - **File control** — `sqlite.FileControl` interface (`PersistWAL`,
     `DataVersion`)
   - **Runtime limits** — `sqlite.Limit` (sqlite3_limit)
   - **Column metadata** — `sqlite.QueryColumnInfo` (name, declared type,
     source database/table/column)
   - **Custom connector** — `sqlite.NewConnector` for `sql.OpenDB` without
     process-global `sql.Register`

# Releases
- Latest: **v1.14.0**
- Pure-Go SQLite driver for GORM, requires Go **1.25+**.

