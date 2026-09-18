package jsonrpc

import (
	"github.com/komari-monitor/komari/pkg/rpc"
	"strings"
	"testing"
)

func TestAllAdminMethodsRejectGuestAndAgent(t *testing.T) {
	n := 0
	for _, method := range rpc.ListMethods() {
		if !strings.HasPrefix(method, "admin:") {
			continue
		}
		n++
		for _, p := range []*rpc.Principal{rpc.NewAnonymousPrincipal(), rpc.NewAgentPrincipal("node")} {
			if rpc.CheckPrincipal(p, method) {
				t.Errorf("unauthorized method %s", method)
			}
		}
	}
	if n < 30 {
		t.Fatalf("incomplete registry coverage: %d", n)
	}
	if !rpc.IsSensitive("admin:exec") {
		t.Fatal("remote execution missing 2FA policy")
	}
}
