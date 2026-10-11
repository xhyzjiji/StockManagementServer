// Package db 负责 MySQL 连接与按版本顺序执行的结构迁移。
// Package db opens MySQL and applies versioned schema migrations in order.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"private/construction_warehouse/internal/config"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Open(cfg config.MySQLConfig) (*sql.DB, error) {
	conn, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(cfg.MaxOpenConns)
	conn.SetMaxIdleConns(cfg.MaxIdleConns)
	conn.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return conn, nil
}

// Migrate 依次执行 migrations/NNN_*.sql 中尚未执行过的版本。
// 约定：已发布的迁移文件不可修改，结构变更一律新增更高版本号的文件。
//
// Migrate applies every migrations/NNN_*.sql version that has not run yet.
// Convention: released migration files are immutable; schema changes go into a new, higher-numbered file.
func Migrate(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INT NOT NULL PRIMARY KEY,
		applied_at BIGINT NOT NULL
	) ENGINE=InnoDB`); err != nil {
		return err
	}
	applied := map[int]bool{}
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	type migration struct {
		version int
		name    string
	}
	var list []migration
	for _, e := range entries {
		prefix, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			continue
		}
		v, err := strconv.Atoi(prefix)
		if err != nil {
			return fmt.Errorf("bad migration file name %s", e.Name())
		}
		list = append(list, migration{v, e.Name()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].version < list[j].version })

	for _, m := range list {
		if applied[m.version] {
			continue
		}
		raw, err := migrationFS.ReadFile("migrations/" + m.name)
		if err != nil {
			return err
		}
		// MySQL 的 DDL 会隐式提交，无法整体回滚；单条失败时直接报错退出，修复后重跑
		// MySQL DDL commits implicitly and cannot be rolled back as a whole; fail fast and rerun after fixing.
		for _, stmt := range splitStatements(string(raw)) {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s: %w\n%s", m.name, err, stmt)
			}
		}
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`,
			m.version, time.Now().UnixMilli()); err != nil {
			return err
		}
	}
	return nil
}

// splitStatements 按分号切分 SQL，并去掉 "--" 行注释；迁移文件中不允许在字符串里出现分号。
// splitStatements splits SQL on semicolons and strips "--" line comments; semicolons inside string literals are not supported.
func splitStatements(sqlText string) []string {
	var cleaned strings.Builder
	for _, line := range strings.Split(sqlText, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		cleaned.WriteString(line)
		cleaned.WriteByte('\n')
	}
	var out []string
	for _, s := range strings.Split(cleaned.String(), ";") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
