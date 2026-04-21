/**
 * 数据库方言（与 sql.Open 的 driverName 对应），用于建表与占位符差异
 */
package db

import "strings"

type Dialect int

const (
	DialectPostgres Dialect = iota
	DialectMySQL
	DialectSQLite
)

func DialectFromDriver(driver string) Dialect {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mysql", "mariadb":
		return DialectMySQL
	case "sqlite", "sqlite3":
		return DialectSQLite
	case "postgres", "postgresql", "pg":
		return DialectPostgres
	default:
		return DialectPostgres
	}
}

func (d Dialect) String() string {
	switch d {
	case DialectMySQL:
		return "mysql"
	case DialectSQLite:
		return "sqlite"
	default:
		return "postgres"
	}
}
