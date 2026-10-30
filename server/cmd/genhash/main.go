// 生成 bcrypt 哈希的小工具：go run ./cmd/genhash <明文>
package main

import (
	"fmt"
	"os"

	"canteen/server/internal/auth"
)

func main() {
	plain := "admin123"
	if len(os.Args) > 1 {
		plain = os.Args[1]
	}
	h, err := auth.HashPassword(plain)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(h)
}
