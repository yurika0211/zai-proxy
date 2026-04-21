/**
 * sql.DB 连接池与 SQLite PRAGMA（按方言）
 */
package db

import (
	"database/sql"
	"time"

	"codex-proxy/internal/config"
)

/* ConfigurePool 在 Ping 成功后调用；SQLite 固定单连接并启用 WAL */
func ConfigurePool(db *sql.DB, d Dialect, cfg *config.Config) {
	if db == nil {
		return
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	switch d {
	case DialectSQLite:
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)
		db.SetConnMaxIdleTime(0)
		_, _ = db.Exec(`PRAGMA busy_timeout=8000`)
		_, _ = db.Exec(`PRAGMA journal_mode=WAL`)
		_, _ = db.Exec(`PRAGMA synchronous=NORMAL`)
		return
	}

	open := cfg.DBMaxOpenConns
	idle := cfg.DBMaxIdleConns
	if open <= 0 {
		rc := cfg.RefreshConcurrency
		if rc < 16 {
			rc = 16
		}
		open = rc
		if open > 128 {
			open = 128
		}
	}
	if open > 256 {
		open = 256
	}
	if open < 4 {
		open = 4
	}
	if idle <= 0 {
		idle = open / 2
		if idle < 4 {
			idle = 4
		}
	}
	if idle > open {
		idle = open
	}

	life := time.Duration(cfg.DBConnMaxLifetimeSec) * time.Second
	if cfg.DBConnMaxLifetimeSec <= 0 {
		life = 30 * time.Minute
	}
	if life > 2*time.Hour {
		life = 2 * time.Hour
	}

	db.SetMaxOpenConns(open)
	db.SetMaxIdleConns(idle)
	db.SetConnMaxLifetime(life)
	db.SetConnMaxIdleTime(10 * time.Minute)
}
