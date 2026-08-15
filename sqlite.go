package sqlite

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"gorm.io/gorm/callbacks"

	msqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
)

// addTimeFormatParam appends DSN parameters to modernc.org/sqlite so it
// matches the behaviour of glebarez/go-sqlite (which GORM's test suite
// was originally written against):
//   - _time_format=sqlite: write time values as parseTimeFormats[0]
//     ("2006-01-02 15:04:05.999999999-07:00") instead of t.String().
//   - _texttotime=1: report time.Time as the ScanType for TEXT columns
//     declared as DATE/DATETIME/TIME/TIMESTAMP, so GORM prepares
//     *time.Time scan targets instead of *string (preventing
//     convertAssign from re-formatting to RFC3339).
//   - _busy_timeout=5000: set per-connection busy timeout (not just on
//     the first connection).
//   - _journal_mode=wal: enable WAL for read concurrency when multiple
//     connections are open (needed for GORM's PreparedStmt LRU and
//     concurrent association appends).
//
// If the DSN already sets the respective parameter it is left untouched.
func addTimeFormatParam(dsn string) string {
	var params []string
	if !strings.Contains(dsn, "_time_format=") {
		params = append(params, "_time_format=sqlite")
	}
	if !strings.Contains(dsn, "_texttotime=") {
		params = append(params, "_texttotime=1")
	}
	if !strings.Contains(dsn, "_busy_timeout=") && !strings.Contains(dsn, "_timeout=") {
		params = append(params, "_busy_timeout=5000")
	}
	if !strings.Contains(dsn, "_journal_mode=") && !strings.Contains(dsn, "_journal=") {
		params = append(params, "_journal_mode=wal")
	}
	if len(params) == 0 {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + strings.Join(params, "&")
}

// DriverName is the default driver name for SQLite.
const DriverName = "sqlite"

type Dialector struct {
	DriverName string
	DSN        string
	Conn       gorm.ConnPool
}

type Config struct {
	DriverName string
	DSN        string
	Conn       gorm.ConnPool
}

func Open(dsn string) gorm.Dialector {
	return &Dialector{DSN: dsn}
}

func New(config Config) gorm.Dialector {
	return &Dialector{DSN: config.DSN, DriverName: config.DriverName, Conn: config.Conn}
}

func (dialector Dialector) Name() string {
	return "sqlite"
}

func (dialector Dialector) Initialize(db *gorm.DB) (err error) {
	if dialector.DriverName == "" {
		dialector.DriverName = DriverName
	}

	if dialector.Conn != nil {
		db.ConnPool = dialector.Conn
	} else {
		dsn := addTimeFormatParam(dialector.DSN)
		conn, err := sql.Open(dialector.DriverName, dsn)
		if err != nil {
			return err
		}
		db.ConnPool = conn
		// busy_timeout is set per-connection via the _busy_timeout DSN
		// parameter (see addTimeFormatParam). The default connection pool
		// size is left untouched so GORM's PreparedStmt LRU background
		// goroutine and concurrent association appends don't deadlock on
		// a single-conn pool.
	}

	var version string
	if err := db.ConnPool.QueryRowContext(context.Background(), "select sqlite_version()").Scan(&version); err != nil {
		return err
	}
	// https://www.sqlite.org/releaselog/3_35_0.html
	if compareVersion(version, "3.35.0") >= 0 {
		callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
			CreateClauses:        []string{"INSERT", "VALUES", "ON CONFLICT", "RETURNING"},
			UpdateClauses:        []string{"UPDATE", "SET", "FROM", "WHERE", "RETURNING"},
			DeleteClauses:        []string{"DELETE", "FROM", "WHERE", "RETURNING"},
			LastInsertIDReversed: true,
		})
	} else {
		callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
			LastInsertIDReversed: true,
		})
	}

	for k, v := range dialector.ClauseBuilders() {
		if _, ok := db.ClauseBuilders[k]; !ok {
			db.ClauseBuilders[k] = v
		}
	}
	return
}

func (dialector Dialector) ClauseBuilders() map[string]clause.ClauseBuilder {
	return map[string]clause.ClauseBuilder{
		"INSERT": func(c clause.Clause, builder clause.Builder) {
			if insert, ok := c.Expression.(clause.Insert); ok {
				if stmt, ok := builder.(*gorm.Statement); ok {
					stmt.WriteString("INSERT ")
					if insert.Modifier != "" {
						stmt.WriteString(insert.Modifier)
						stmt.WriteByte(' ')
					}

					stmt.WriteString("INTO ")
					if insert.Table.Name == "" {
						stmt.WriteQuoted(stmt.Table)
					} else {
						stmt.WriteQuoted(insert.Table)
					}
					return
				}
			}

			c.Build(builder)
		},
		"LIMIT": func(c clause.Clause, builder clause.Builder) {
			if limit, ok := c.Expression.(clause.Limit); ok {
				var lmt = -1
				if limit.Limit != nil && *limit.Limit >= 0 {
					lmt = *limit.Limit
				}
				if lmt >= 0 || limit.Offset > 0 {
					builder.WriteString("LIMIT ")
					builder.WriteString(strconv.Itoa(lmt))
				}
				if limit.Offset > 0 {
					builder.WriteString(" OFFSET ")
					builder.WriteString(strconv.Itoa(limit.Offset))
				}
			}
		},
		"FOR": func(c clause.Clause, builder clause.Builder) {
			if _, ok := c.Expression.(clause.Locking); ok {
				// SQLite3 does not support row-level locking.
				return
			}
			c.Build(builder)
		},
	}
}

func (dialector Dialector) DefaultValueOf(field *schema.Field) clause.Expression {
	if field.AutoIncrement {
		return clause.Expr{SQL: "NULL"}
	}

	// doesn't work, will raise error
	return clause.Expr{SQL: "DEFAULT"}
}

func (dialector Dialector) Migrator(db *gorm.DB) gorm.Migrator {
	return Migrator{migrator.Migrator{Config: migrator.Config{
		DB:                          db,
		Dialector:                   dialector,
		CreateIndexAfterCreateTable: true,
	}}}
}

func (dialector Dialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {
	writer.WriteByte('?')
}

func (dialector Dialector) QuoteTo(writer clause.Writer, str string) {
	var (
		underQuoted, selfQuoted bool
		continuousBacktick      int8
		shiftDelimiter          int8
	)

	for _, v := range []byte(str) {
		switch v {
		case '`':
			continuousBacktick++
			if continuousBacktick == 2 {
				writer.WriteString("``")
				continuousBacktick = 0
			}
		case '.':
			if continuousBacktick > 0 || !selfQuoted {
				shiftDelimiter = 0
				underQuoted = false
				continuousBacktick = 0
				writer.WriteString("`")
			}
			writer.WriteByte(v)
			continue
		default:
			if shiftDelimiter-continuousBacktick <= 0 && !underQuoted {
				writer.WriteString("`")
				underQuoted = true
				if selfQuoted = continuousBacktick > 0; selfQuoted {
					continuousBacktick -= 1
				}
			}

			for ; continuousBacktick > 0; continuousBacktick -= 1 {
				writer.WriteString("``")
			}

			writer.WriteByte(v)
		}
		shiftDelimiter++
	}

	if continuousBacktick > 0 && !selfQuoted {
		writer.WriteString("``")
	}
	writer.WriteString("`")
}

func (dialector Dialector) Explain(sql string, vars ...interface{}) string {
	return logger.ExplainSQL(sql, nil, `"`, vars...)
}

func (dialector Dialector) DataTypeOf(field *schema.Field) string {
	if expr, ok := generatedColumnExpr(field); ok {
		return dialector.dataTypeOf(field) + " GENERATED ALWAYS AS (" + expr + ") STORED"
	}

	return dialector.dataTypeOf(field)
}

func (dialector Dialector) dataTypeOf(field *schema.Field) string {
	switch field.DataType {
	case schema.Bool:
		return "numeric"
	case schema.Int, schema.Uint:
		if field.AutoIncrement {
			// doesn't check `PrimaryKey`, to keep backward compatibility
			// https://www.sqlite.org/autoinc.html
			return "integer PRIMARY KEY AUTOINCREMENT"
		} else {
			return "integer"
		}
	case schema.Float:
		return "real"
	case schema.String:
		return "text"
	case schema.Time:
		// Distinguish between schema.Time and tag time
		if val, ok := field.TagSettings["TYPE"]; ok {
			return val
		} else {
			return "datetime"
		}
	case schema.Bytes:
		return "blob"
	}

	return string(field.DataType)
}

func (dialectopr Dialector) SavePoint(tx *gorm.DB, name string) error {
	return tx.Exec("SAVEPOINT " + name).Error
}

func (dialectopr Dialector) RollbackTo(tx *gorm.DB, name string) error {
	return tx.Exec("ROLLBACK TO SAVEPOINT " + name).Error
}

func (dialector Dialector) Translate(err error) error {
	switch terr := err.(type) {
	case *msqlite.Error:
		switch terr.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE:
			return gorm.ErrDuplicatedKey
		case sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return gorm.ErrDuplicatedKey
		case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
			return gorm.ErrForeignKeyViolated
		case sqlite3.SQLITE_CONSTRAINT_CHECK:
			return gorm.ErrCheckConstraintViolated
		}
	}
	return err
}

func compareVersion(version1, version2 string) int {
	n, m := len(version1), len(version2)
	i, j := 0, 0
	for i < n || j < m {
		x := 0
		for ; i < n && version1[i] != '.'; i++ {
			x = x*10 + int(version1[i]-'0')
		}
		i++
		y := 0
		for ; j < m && version2[j] != '.'; j++ {
			y = y*10 + int(version2[j]-'0')
		}
		j++
		if x > y {
			return 1
		}
		if x < y {
			return -1
		}
	}
	return 0
}

// generatedColumnExpr returns the expression of a computed (generated) column
// declared via the `generated` tag, if any. The `identity` keyword is reserved
// for identity columns (rendered through the dialect's native auto-increment)
// and is not a computed-column expression.
func generatedColumnExpr(field *schema.Field) (string, bool) {
	value, ok := field.TagSettings["GENERATED"]
	if !ok {
		return "", false
	}
	// Ignore an empty value or a bare `generated` tag, which the tag parser
	// stores as the upper-cased key, rather than treating it as an expression.
	if value = strings.TrimSpace(value); value == "" || value == "GENERATED" {
		return "", false
	}
	if isIdentityKeyword(value) {
		return "", false
	}
	return value, true
}

// isIdentityKeyword reports whether value is the `identity` keyword, optionally
// combined with the generation mode `always` / `by default`. Any other token
// means value is a computed-column expression.
func isIdentityKeyword(value string) bool {
	identity := false
	for _, token := range strings.Fields(strings.ToLower(value)) {
		switch token {
		case "identity":
			identity = true
		case "always", "by", "default":
		default:
			return false
		}
	}
	return identity
}
