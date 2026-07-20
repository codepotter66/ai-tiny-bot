// Package store 提供 SQLite 连接、迁移、仓储方法。
//
// 设计：
//   - 纯 Go 驱动（modernc.org/sqlite），无 CGo
//   - 单文件 DB，WAL 模式
//   - 写连接 SetMaxOpenConns(1)，避免 "database is locked"
//   - 读连接池独立（SetMaxOpenConns=N），通过 RW() 获取
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

const schemaSQL = "schema.sql"

// Store 包装读写两个 *sql.DB。
type Store struct {
	RW *sql.DB // 写连接池（max 1）
	RO *sql.DB // 读连接池（max N）
}

// Open 打开 dbPath，自动建目录、启用 WAL、跑迁移。
func Open(ctx context.Context, dbPath string) (*Store, error) {
	if dbPath == "" {
		return nil, errors.New("store: dbPath empty")
	}
	// 父目录
	if dir := filepath.Dir(dbPath); dir != "" {
		if err := mkdirAll(dir); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	// 写连接
	rw, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open rw db: %w", err)
	}
	rw.SetMaxOpenConns(1)
	if err := rw.PingContext(ctx); err != nil {
		_ = rw.Close()
		return nil, fmt.Errorf("ping rw: %w", err)
	}

	// 读连接
	ro, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		_ = rw.Close()
		return nil, fmt.Errorf("open ro db: %w", err)
	}
	ro.SetMaxOpenConns(4)
	ro.SetMaxIdleConns(2)

	s := &Store{RW: rw, RO: ro}
	if err := s.migrate(ctx); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close 关闭两个连接池。
func (s *Store) Close() error {
	var errs []error
	if s.RW != nil {
		if err := s.RW.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.RO != nil {
		if err := s.RO.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// migrate 跑 schema.sql（CREATE IF NOT EXISTS）并补齐增量列。
func (s *Store) migrate(ctx context.Context) error {
	raw, err := schemaFS.ReadFile(schemaSQL)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	tx, err := s.RW.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, string(raw)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply schema: %w", err)
	}
	// 旧库 users 表可能没有 soul_md；新建表已含该列时 ALTER 会报 duplicate，忽略即可。
	if _, err := tx.ExecContext(ctx, `ALTER TABLE users ADD COLUMN soul_md TEXT`); err != nil {
		if !isSQLiteDuplicateColumn(err) {
			_ = tx.Rollback()
			return fmt.Errorf("add soul_md: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO schema_version(version) VALUES (2)"); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record schema_version: %w", err)
	}
	return tx.Commit()
}

func isSQLiteDuplicateColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column")
}

// mkdirAll 是 os.MkdirAll 的小包装，方便单测替换。
var mkdirAll = func(dir string) error { return osMkdirAll(dir) }
