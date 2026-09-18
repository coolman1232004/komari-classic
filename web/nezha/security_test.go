package nezha

import (
	"context"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/web/nezha/proto"
	"google.golang.org/grpc/metadata"
	"testing"
)

func TestCompatRequiresProvisionedNodeAndCurrentToken(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:nezha_security?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	node := models.Client{UUID: "known-node", Token: "valid-secret", Name: "admin name"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	defer db.Delete(&node)
	ctx := func(id, secret string) context.Context {
		return metadata.NewIncomingContext(context.Background(), metadata.Pairs("client_uuid", id, "client_secret", secret))
	}
	server := &nezhaCompatServer{}
	for _, pair := range [][2]string{{"known-node", "wrong-secret"}, {"x", "attacker-key"}} {
		c := ctx(pair[0], pair[1])
		if _, _, err := getAuth(c); err == nil {
			t.Fatal("accepted invalid credentials")
		}
		if _, err := server.ReportSystemInfo(c, &proto.Host{}); err == nil {
			t.Fatal("invalid host upload accepted")
		}
		if _, err := server.ReportGeoIP(c, &proto.GeoIP{}); err == nil {
			t.Fatal("invalid geo upload accepted")
		}
	}
	if err := ingestState("x", &proto.State{}); err == nil {
		t.Fatal("unknown state accepted")
	}
	if err := upsertClientFromHost("x", "attacker-key", &proto.Host{}); err == nil {
		t.Fatal("unprovisioned node created")
	}
	c := ctx(node.UUID, node.Token)
	if _, err := server.ReportSystemInfo(c, &proto.Host{Platform: "Linux"}); err != nil {
		t.Fatal(err)
	}
	var got models.Client
	db.First(&got, "uuid = ?", node.UUID)
	if got.Name != node.Name || got.Token != node.Token || got.OS != "Linux" {
		t.Fatalf("bad update: %+v", got)
	}
	if err := db.Model(&node).Update("token", "rotated-secret").Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := getAuth(c); err == nil {
		t.Fatal("rotated token still accepted")
	}
	var count int64
	db.Model(&models.Client{}).Where("uuid = ?", "x").Count(&count)
	if count != 0 {
		t.Fatal("unauthorized registration persisted")
	}
}
