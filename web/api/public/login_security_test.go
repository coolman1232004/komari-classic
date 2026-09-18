package public

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
)

func TestLegacyPasswordMigrationAndReset(t *testing.T) {
	user, err := accounts.CreateAccount("migration-user", "temporary")
	if err != nil {
		t.Fatal(err)
	}
	db := dbcore.GetDBInstance()
	sum := sha256.Sum256([]byte("old-password06Wm4Jv1Hkxx"))
	legacy := base64.StdEncoding.EncodeToString(sum[:])
	if err := db.Model(&models.User{}).Where("uuid = ?", user.UUID).Update("passwd", legacy).Error; err != nil {
		t.Fatal(err)
	}
	if _, ok := accounts.CheckPassword(user.Username, "wrong"); ok {
		t.Fatal("wrong legacy password accepted")
	}
	unchanged, _ := accounts.GetUserByUUID(user.UUID)
	if unchanged.Passwd != legacy {
		t.Fatal("failed verification changed hash")
	}
	if id, ok := accounts.CheckPassword(user.Username, "old-password"); !ok || id != user.UUID {
		t.Fatal("legacy login failed")
	}
	migrated, _ := accounts.GetUserByUUID(user.UUID)
	if !strings.HasPrefix(migrated.Passwd, "$argon2id$") {
		t.Fatal("hash not migrated")
	}
	if _, ok := accounts.CheckPassword(user.Username, "old-password"); !ok {
		t.Fatal("migrated password rejected")
	}
	if err := accounts.ForceResetPassword(user.Username, "reset-password"); err != nil {
		t.Fatal(err)
	}
	if _, ok := accounts.CheckPassword(user.Username, "old-password"); ok {
		t.Fatal("old password accepted after reset")
	}
	if _, ok := accounts.CheckPassword(user.Username, "reset-password"); !ok {
		t.Fatal("reset password rejected")
	}
	replacement := "changed-password"
	if err := accounts.UpdateUser(user.UUID, nil, &replacement, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := accounts.CheckPassword(user.Username, replacement); !ok {
		t.Fatal("changed password rejected")
	}
}

func TestLoginLimiterExpiryAndAccountBudget(t *testing.T) {
	l := &loginLimiter{windows: make(map[[32]byte]loginWindow)}
	now := time.Now()
	for i := 0; i < 10; i++ {
		if !l.allow(fmt.Sprintf("192.0.2.%d:1234", i), "target", now) {
			t.Fatal("premature limit")
		}
	}
	if l.allow("198.51.100.1:1234", "target", now) {
		t.Fatal("IP rotation bypassed account budget")
	}
	if !l.allow("192.0.2.1:1234", "target", now.Add(5*time.Minute)) {
		t.Fatal("limit did not expire")
	}
}

func TestLoginLimiterGlobalPeerAndCapacity(t *testing.T) {
	now := time.Now()
	l := &loginLimiter{windows: make(map[[32]byte]loginWindow)}
	for i := 0; i < 20; i++ {
		if !l.allow("192.0.2.1:1234", fmt.Sprint(i), now) {
			t.Fatal("premature peer limit")
		}
	}
	if l.allow("192.0.2.1:5678", "other", now) {
		t.Fatal("source port bypassed peer budget")
	}
	l = &loginLimiter{windows: make(map[[32]byte]loginWindow)}
	for i := 0; i < 60; i++ {
		if !l.allow(fmt.Sprintf("192.0.2.%d:1234", i), fmt.Sprint(i), now) {
			t.Fatal("premature global limit")
		}
	}
	if l.allow("203.0.113.1:1234", "other", now) {
		t.Fatal("global budget bypassed")
	}
	l = &loginLimiter{windows: make(map[[32]byte]loginWindow)}
	for i := 0; i < 4096; i++ {
		l.windows[sha256.Sum256([]byte(fmt.Sprint(i)))] = loginWindow{1, now.Add(time.Minute)}
	}
	if l.allow("203.0.113.1:1234", "other", now) || len(l.windows) > 4096 {
		t.Fatal("capacity not bounded")
	}
}

func TestLoginThrottlesTwoFactorAndSpoofedHeaders(t *testing.T) {
	old := passwordLoginLimiter
	passwordLoginLimiter = &loginLimiter{windows: make(map[[32]byte]loginWindow)}
	defer func() { passwordLoginLimiter = old }()
	user, err := accounts.CreateAccount("throttle-user", "known-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := dbcore.GetDBInstance().Model(&models.User{}).Where("uuid = ?", user.UUID).Update("two_factor", "JBSWY3DPEHPK3PXP").Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/api/login", Login)
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"throttle-user","password":"known-password"}`))
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		want := http.StatusUnauthorized
		if i == 10 {
			want = http.StatusTooManyRequests
			if w.Header().Get("Retry-After") == "" {
				t.Fatal("missing retry hint")
			}
		}
		if w.Code != want {
			t.Fatalf("attempt %d: got %d, want %d: %s", i, w.Code, want, w.Body.String())
		}
		if w.Header().Get("Set-Cookie") != "" {
			t.Fatal("unauthorized attempt created cookie")
		}
	}
}
