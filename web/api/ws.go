package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/protocol/v1"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/connection"
)

func GetClients(c *gin.Context) {
	// 升级到ws
	if !IsWebSocketUpgrade(c) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Require WebSocket upgrade"})
		return
	}
	// Upgrade the HTTP connection to a WebSocket connection
	raw, err := UpgradeWebSocket(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Failed to upgrade to WebSocket." + err.Error()})
		return
	}
	conn := connection.NewSafeConn(raw)
	defer conn.Close()

	// 请求
	for {
		var resp struct {
			Online []string             `json:"online"` // 已建立连接的客户端uuid列表
			Data   map[string]v1.Report `json:"data"`   // 最后上报的数据
		}

		resp.Online = []string{}
		resp.Data = map[string]v1.Report{}

		_, data, err := conn.ReadMessage()
		if err != nil {
			//log.Println("Error reading message:", err)
			return
		}
		principal := IdentifyPrincipal(c)
		isLogin := principal.HasRole(rpc.RoleAdmin)
		if !CanReadLiveData(c) {
			return
		}
		visible := map[string]bool{}
		if !isLogin {
			var nodes []models.Client
			if err := dbcore.GetDBInstance().Select("uuid", "hidden").Find(&nodes).Error; err != nil {
				return
			}
			for _, node := range nodes {
				visible[node.UUID] = !node.Hidden
			}
		}
		message := string(data)

		uuID := ""
		if message != "get" { // 非请求全部内容
			if strings.HasPrefix(message, "get ") {
				uuID = strings.TrimSpace(strings.TrimPrefix(message, "get "))
			} else {
				conn.WriteJSON(gin.H{"status": "error", "error": "Invalid message"})
				continue
			}
		}

		// 在线客户端uuid列表（WebSocket 与非 WebSocket）
		for _, key := range agent_runtime.GetAllOnlineUUIDs() {
			if !isLogin && !visible[key] {
				continue
			}
			if uuID != "" && key != uuID {
				continue
			}
			resp.Online = append(resp.Online, key)
		}

		//过往节点数据信息
		for key, report := range agent_runtime.GetLatestReport() {
			if !isLogin && !visible[key] {
				continue
			}
			if uuID != "" && key != uuID {
				continue
			}

			copyReport := *report
			copyReport.UUID = "" // 不暴露 uuid
			if copyReport.CPU.Usage == 0 {
				copyReport.CPU.Usage = 0.01
			}
			resp.Data[key] = copyReport
		}

		err = conn.WriteJSON(gin.H{"status": "success", "data": resp})
		if err != nil {
			return
		}
	}
}

// Re-evaluate private-site and share/session credentials while streaming.
func CanReadLiveData(c *gin.Context) bool {
	private, err := config.GetAs[bool](config.PrivateSiteKey, false)
	if err != nil {
		return false
	}
	return !private || IdentifyPrincipal(c).Type != rpc.PrincipalAnonymous || hasTempAccess(c)
}
