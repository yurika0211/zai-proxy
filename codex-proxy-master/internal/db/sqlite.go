/**
 * SQLite（文件库），DSN 未配置时使用 db-name 或默认 codex_proxy.db
 */
package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"codex-proxy/internal/config"

	_ "modernc.org/sqlite"
)

func OpenSQLite(cfg *config.Config) (*sql.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	dsn := strings.TrimSpace(cfg.DBDSN)
	if dsn == "" {
		path := strings.TrimSpace(cfg.DBName)
		if path == "" {
			path = "codex_proxy.db"
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("sqlite 路径: %w", err)
		}
		/* modernc.org/sqlite：直接使用本地文件路径 */
		dsn = abs
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
