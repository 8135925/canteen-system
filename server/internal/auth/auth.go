package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"golang.org/x/crypto/bcrypt"
)

// Claims JWT 载荷：uid、role、mcp(must_change_password)。
type Claims struct {
	UID  int64  `json:"uid"`
	Role string `json:"role"`
	MCP  bool   `json:"mcp"`
	jwt.RegisteredClaims
}

const (
	TTLRemember = 30 * 24 * time.Hour // 记住登录 30 天
	TTLSession  = 12 * time.Hour      // 默认 12 小时
	BcryptCost  = 10                  // 对齐 sheji.md §8
)

// HashPassword bcrypt 哈希（cost=10）。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword 校验密码。
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// IssueToken 签发 JWT。
func IssueToken(secret string, uid int64, role string, mcp bool, remember bool) (string, error) {
	now := time.Now()
	ttl := TTLSession
	if remember {
		ttl = TTLRemember
	}
	claims := &Claims{
		UID:  uid,
		Role: role,
		MCP:  mcp,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken 解析并校验 JWT。
func ParseToken(secret, token string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(token, &Claims{}, func(tk *jwt.Token) (any, error) {
		if _, ok := tk.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", tk.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
