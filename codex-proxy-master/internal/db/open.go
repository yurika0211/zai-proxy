/**
 * 按 db-driver 打开对应数据库
 */
package db

import (
	"database/sql"
	"fmt"

	"codex-proxy/internal/config"
)

func Open(cfg *config.Config) (*sql.DB, Dialect, error) {
	if cfg == nil {
		return nil, DialectPostgres, fmt.Errorf("config is nil")
	}
	d := DialectFromDriver(cfg.DBDriver)
	var db *sql.DB
	var err error
	switch d {
	case DialectMySQL:
		db, err = OpenMySQL(cfg)
	case DialectSQLite:
		db, err = OpenSQLite(cfg)
	default:
		d = DialectPostgres
		db, err = OpenPostgres(cfg)
	}
	if err != nil {
		return nil, d, err
	}
	ConfigurePool(db, d, cfg)
	return db, d, nil
}
