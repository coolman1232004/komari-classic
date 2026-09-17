package client

import (
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"testing"
)

func TestBasicInfoCannotModifyAdminFields(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:basic_security?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	node := models.Client{UUID: "basic-security", Token: "original-secret", Name: "admin-name", Hidden: true, Price: 42}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	defer db.Delete(&node)
	err := ingestBasicInfo(node.UUID, map[string]interface{}{"uuid": "another-node", "token": "attacker-secret", "Token": "case-bypass", "name": "attacker-name", "hidden": false, "price": 0, "region": "fake", "cpu_name": "legitimate-cpu", "cpu_cores": 4}, "")
	if err != nil {
		t.Fatal(err)
	}
	var got models.Client
	if err := db.First(&got, "uuid = ?", node.UUID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Token != node.Token || got.Name != node.Name || !got.Hidden || got.Price != 42 || got.Region != "" {
		t.Fatalf("administrator fields overwritten: %+v", got)
	}
	if got.CpuName != "legitimate-cpu" || got.CpuCores != 4 {
		t.Fatal("legitimate telemetry lost")
	}
}
