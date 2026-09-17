package api

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/pkg/rpc"
)

// Capture credentials, not a pooled gin.Context, for long-lived connections.
func CredentialValidator(c *gin.Context) func() bool {
	p := IdentifyPrincipal(c)
	auth := c.GetHeader("Authorization")
	session, _ := c.Cookie("session_token")
	token := extractClientToken(c)
	return func() bool {
		switch p.Type {
		case rpc.PrincipalAPIKey:
			return isApiKeyValid(auth)
		case rpc.PrincipalUser:
			id, err := accounts.GetSession(session)
			return err == nil && id == p.UserUUID
		case rpc.PrincipalAgent:
			id, err := checkTokenAndGetUUID(token)
			return err == nil && id != "" && id == p.ClientUUID
		default:
			return false
		}
	}
}
