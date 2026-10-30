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
	// 连接串只从环境变量读取，代码中不落任何真实凭据
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "missing env DATABASE_URL (e.g. set it to your Supabase Session Pooler connection string)")
		os.Exit(1)
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
