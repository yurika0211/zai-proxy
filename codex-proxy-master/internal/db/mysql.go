/**
 * MySQL 连接；库不存在时尝试创建后重连
 */
package db

import (
	"database/sql"
	"fmt"
	"strings"

	"codex-proxy/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

func OpenMySQL(cfg *config.Config) (*sql.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	targetName := strings.TrimSpace(cfg.DBName)
	if targetName == "" {
		targetName = "codex_proxy"
	}
	dsn := strings.TrimSpace(cfg.DBDSN)
	if dsn != "" {
		dsn = enrichMySQLDSN(dsn)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		if err = db.Ping(); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	}

	user := cfg.DBUser
	pass := cfg.DBPassword
	host := cfg.DBHost
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.DBPort
	if port == 0 {
		port = 3306
	}

	openNamed := func(dbName string) (*sql.DB, error) {
		u := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=false&collation=utf8mb4_unicode_ci&timeout=12s&readTimeout=120s&writeTimeout=120s",
			user, pass, host, port, dbName)
		db, err := sql.Open("mysql", u)
		if err != nil {
			return nil, err
		}
		if err = db.Ping(); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	}

	db, err := openNamed(targetName)
	if err == nil {
		return db, nil
	}
	errStr := strings.ToLower(err.Error())
	if !strings.Contains(errStr, "1049") && !strings.Contains(errStr, "unknown database") {
		return nil, err
	}

	adminDB, err := openNamed("")
	if err != nil {
		return nil, fmt.Errorf("MySQL 管理连接失败: %w", err)
	}
	defer adminDB.Close()

	q := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
		mysqlBacktickIdent(targetName),
	)
	if _, err = adminDB.Exec(q); err != nil {
		return nil, fmt.Errorf("创建 MySQL 库失败: %w", err)
	}
	return openNamed(targetName)
}

func mysqlBacktickIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func enrichMySQLDSN(dsn string) string {
	if strings.Contains(strings.ToLower(dsn), "timeout=") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		if strings.HasSuffix(dsn, "?") || strings.HasSuffix(dsn, "&") {
			return dsn + "timeout=12s&readTimeout=120s&writeTimeout=120s"
		}
		return dsn + "&timeout=12s&readTimeout=120s&writeTimeout=120s"
	}
	return dsn + "?timeout=12s&readTimeout=120s&writeTimeout=120s"
}
