/**
 * PostgreSQL 连接与建库（库不存在时尝试用 postgres 库创建后重连）
 */
package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"codex-proxy/internal/config"

	_ "github.com/lib/pq"
)

func OpenPostgres(cfg *config.Config) (*sql.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	targetName := strings.TrimSpace(cfg.DBName)
	if targetName == "" {
		targetName = "codex_proxy"
	}

	dsn := strings.TrimSpace(cfg.DBDSN)
	if dsn == "" {
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, targetName, cfg.DBSSLMode)
	}
	dsn = enrichPostgresDSN(dsn)

	openDB := func(uri string) (*sql.DB, error) {
		db, err := sql.Open("postgres", uri)
		if err != nil {
			return nil, err
		}
		if err = db.Ping(); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	}

	db, err := openDB(dsn)
	if err == nil {
		return db, nil
	}

	if !strings.Contains(err.Error(), "does not exist") && !strings.Contains(err.Error(), "不存在") {
		return nil, err
	}

	adminDSN := dsn
	if cfg.DBDSN == "" {
		adminDSN = fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=%s",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBSSLMode)
	} else {
		parsed, parseErr := url.Parse(cfg.DBDSN)
		if parseErr == nil {
			parsed.Path = "/postgres"
			adminDSN = parsed.String()
		}
	}
	adminDSN = enrichPostgresDSN(adminDSN)

	adminDB, err := openDB(adminDSN)
	if err != nil {
		return nil, fmt.Errorf("admin DB 连接失败: %w", err)
	}
	defer adminDB.Close()

	_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", pqQuoteIdent(targetName)))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, fmt.Errorf("创建数据库失败: %w", err)
	}

	return openDB(dsn)
}

func pqQuoteIdent(identifier string) string {
	identifier = strings.ReplaceAll(identifier, `"`, `\"`)
	return `"` + identifier + `"`
}

func enrichPostgresDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	q := u.Query()
	if q.Get("connect_timeout") == "" {
		q.Set("connect_timeout", "12")
	}
	if q.Get("application_name") == "" {
		q.Set("application_name", "codex-proxy")
	}
	u.RawQuery = q.Encode()
	return u.String()
}
