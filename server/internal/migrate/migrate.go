// Package migrate 迁移执行器：按文件名序执行 up 迁移，schema_migrations 记录已应用版本。
// 语义与 golang-migrate 一致（*.up.sql / *.down.sql 成对，幂等执行）。
package migrate

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"canteen/server/migrations"
)

// Up 执行全部未应用的 up 迁移。幂等：已应用的跳过。
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		create table if not exists schema_migrations (
			version text primary key,
			applied_at timestamptz not null default now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	ups, err := migrations.Ups()
	if err != nil {
		return err
	}
	for _, m := range ups {
		var applied bool
		if err := pool.QueryRow(ctx,
			`select exists(select 1 from schema_migrations where version=$1)`, m.Version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, m.SQL); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", m.Version, err)
		}
		if _, err := tx.Exec(ctx, `insert into schema_migrations (version) values ($1)`, m.Version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", m.Version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
