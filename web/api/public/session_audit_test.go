package public

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/web/api/admin"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExpiredSessionCannotBindOAuth(t *testing.T) {
	u, err := accounts.CreateAccount("expired-audit", "password")
	if err != nil {
		t.Fatal(err)
	}
	s, err := accounts.CreateSession(u.UUID, 60, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	dbcore.GetDBInstance().Model(&models.Session{}).Where("session = ?", s).Update("expires", time.Now().Add(-time.Hour))
	if _, err := accounts.GetUserBySession(s); err == nil {
		t.Fatal("expired session resolved user")
	}
}
func TestPasswordRecoveryRevokesSessions(t *testing.T) {
	u, err := accounts.CreateAccount("reset-audit", "password")
	if err != nil {
		t.Fatal(err)
	}
	s, err := accounts.CreateSession(u.UUID, 60, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	if err := accounts.ForceResetPassword(u.Username, "new-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.GetSession(s); err == nil {
		t.Fatal("recovery retained old session")
	}
}
func TestCannotReplaceEnabledTwoFactor(t *testing.T) {
	u, err := accounts.CreateAccount("twofa-audit", "password")
	if err != nil {
		t.Fatal(err)
	}
	accounts.Enable2Fa(u.UUID, "JBSWY3DPEHPK3PXP")
	r := gin.New()
	r.POST("/enable", func(c *gin.Context) { c.Set("uuid", u.UUID); admin.Enable2FA(c) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/enable?code=123456", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d", w.Code)
	}
}
