package auth

import (
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	h, err := HashPassword("admin123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "admin123") {
		t.Fatal("correct password should pass")
	}
	if CheckPassword(h, "wrong") {
		t.Fatal("wrong password should fail")
	}
	// cost=10 哈希前缀
	if len(h) < 7 || h[:4] != "$2a$" || h[4] != '1' || h[5] != '0' {
		t.Fatalf("expected bcrypt cost 10 prefix, got %s", h)
	}
}

func TestJWTRoundTrip(t *testing.T) {
	const secret = "test-secret"
	tok, err := IssueToken(secret, 42, "employee", true, false)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseToken(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.UID != 42 || c.Role != "employee" || !c.MCP {
		t.Fatalf("claims mismatch: %+v", c)
	}
	// 默认 12h
	if time.Until(c.ExpiresAt.Time) > 12*time.Hour+time.Minute {
		t.Fatalf("session token should be ~12h, got %v", time.Until(c.ExpiresAt.Time))
	}
	// 记住登录 30 天
	tok2, _ := IssueToken(secret, 1, "employee", false, true)
	c2, _ := ParseToken(secret, tok2)
	if time.Until(c2.ExpiresAt.Time) < 29*24*time.Hour {
		t.Fatalf("remember token should be ~30d")
	}
	// 错误密钥
	if _, err := ParseToken("other-secret", tok); err == nil {
		t.Fatal("wrong secret should fail")
	}
	// 篡改 token
	if _, err := ParseToken(secret, tok+"x"); err == nil {
		t.Fatal("tampered token should fail")
	}
}

func TestLoginGuard(t *testing.T) {
	g := NewLoginGuard()
	key := "E001|1.2.3.4"
	if g.LockRemaining(key) != 0 {
		t.Fatal("initially not locked")
	}
	for i := 0; i < maxFails-1; i++ {
		g.Fail(key)
	}
	if g.LockRemaining(key) != 0 {
		t.Fatal("4 fails should not lock")
	}
	g.Fail(key)
	if g.LockRemaining(key) <= 0 {
		t.Fatal("5th fail should lock")
	}
	// 其他 key 不受影响
	if g.LockRemaining("E002|1.2.3.4") != 0 {
		t.Fatal("other key should not be locked")
	}
	g.Reset(key)
	if g.LockRemaining(key) != 0 {
		t.Fatal("reset should unlock")
	}
}
