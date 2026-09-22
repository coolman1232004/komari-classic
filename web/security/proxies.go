package security

import (
	"fmt"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

const clientAddressKey = "komari.validated-client-address"

// ConfigureTrustedProxies opts out of Gin's trust-all default. Only explicit
// proxy IPs/CIDRs can affect ClientIP; account and global limits remain in force.
func ConfigureTrustedProxies(r *gin.Engine, value string) error {
	var proxies []string
	if strings.TrimSpace(value) != "" {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			if ip := net.ParseIP(item); ip == nil {
				_, network, err := net.ParseCIDR(item)
				if err != nil {
					return fmt.Errorf("expected comma-separated proxy IPs or CIDRs")
				}
				ones, _ := network.Mask.Size()
				if ones == 0 {
					return fmt.Errorf("trusting every address is not allowed")
				}
			}
			proxies = append(proxies, item)
		}
	}
	r.TrustedPlatform = ""
	r.RemoteIPHeaders = []string{"X-Forwarded-For"}
	if err := r.SetTrustedProxies(proxies); err != nil {
		return err
	}
	r.Use(func(c *gin.Context) {
		c.Set(clientAddressKey, c.ClientIP())
		c.Next()
	})
	return nil
}

// Fall back to the socket peer when used by a router without our configuration,
// rather than inheriting Gin's permissive default for security-critical limits.
func LoginClientAddress(c *gin.Context) string {
	if value, ok := c.Get(clientAddressKey); ok {
		if address, ok := value.(string); ok && net.ParseIP(address) != nil {
			return address
		}
	}
	return c.Request.RemoteAddr
}
