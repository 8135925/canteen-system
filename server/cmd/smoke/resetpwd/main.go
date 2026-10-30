// 一次性工具：把指定账号密码重置为给定明文，并置 must_change_password=true（恢复首登强改密状态）。
// 用法：go run ./cmd/smoke/resetpwd <emp_no> <new_password>
package main

import (
	"context"
	"fmt"
	"os"

	"canteen/server/internal/auth"
	"github.com/jackc/pgx/v5"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: resetpwd <emp_no> <new_password>")
		os.Exit(1)
	}
	empNo, plain := os.Args[1], os.Args[2]
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgresql://postgres.yikizzibtsokolgnnkfi:MyDb2026Secure!789@aws-0-eu-west-1.pooler.supabase.com:5432/postgres?sslmode=require"
	}
	hash, err := auth.HashPassword(plain)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hash:", err)
		os.Exit(1)
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)
	tag, err := conn.Exec(ctx,
		`update users set password_hash=$1, must_change_password=true, updated_at=now() where emp_no=$2`,
		hash, empNo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "update:", err)
		os.Exit(1)
	}
	fmt.Printf("OK rows=%d emp_no=%s password=%s (首登将强制改密)\n", tag.RowsAffected(), empNo, plain)
}
