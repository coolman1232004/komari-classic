package admin

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/utils/backup"
	"github.com/komari-monitor/komari/web/api"
)

var restoreMutex sync.Mutex

// UploadBackup validates completely before queueing a restore. Queued archives
// cannot be overwritten by another request during the restart delay.
func UploadBackup(c *gin.Context) {
	if !restoreMutex.TryLock() {
		api.RespondError(c, 409, "Another restore is in progress")
		return
	}
	defer restoreMutex.Unlock()
	finalPath := filepath.Join("data", backup.PendingName)
	if _, err := os.Lstat(finalPath); !os.IsNotExist(err) {
		api.RespondError(c, 409, "A pending restore already exists")
		return
	}
	if !flags.IsSQLite() || !backup.DefaultDatabase("./data", flags.DatabaseFile) {
		api.RespondError(c, 400, "Restore requires the default data/komari.db database location")
		return
	}
	file, header, err := c.Request.FormFile("backup")
	if err != nil {
		api.RespondError(c, 400, "Cannot read backup upload")
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		api.RespondError(c, 400, "Backup must be a ZIP archive")
		return
	}
	if err := os.MkdirAll("./data", 0700); err != nil {
		api.RespondError(c, 500, "Cannot prepare data directory")
		return
	}
	temp, err := os.CreateTemp("./data", ".backup-upload-*.zip")
	if err != nil {
		api.RespondError(c, 500, "Cannot stage backup upload")
		return
	}
	defer os.Remove(temp.Name())
	_, err = io.Copy(temp, file)
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil || closeErr != nil {
		api.RespondError(c, 500, "Cannot save backup upload")
		return
	}
	stage, err := os.MkdirTemp("", "komari-restore-check-*")
	if err != nil {
		api.RespondError(c, 500, "Cannot prepare backup validation")
		return
	}
	defer os.RemoveAll(stage)
	if err := backup.Stage(temp.Name(), stage); err != nil {
		api.RespondError(c, 400, fmt.Sprintf("Backup rejected: %v", err))
		return
	}
	// Hard linking a fully written file provides an atomic, exclusive publish on
	// the same volume. Fail safely on filesystems that do not support this.
	if err := os.Link(temp.Name(), finalPath); err != nil {
		api.RespondError(c, 500, "Cannot queue restore; check the data volume and pending restore")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Backup validated and queued. The service will restart; sign in with the backup's account after restoration."})
	go func() {
		log.Println("Validated backup queued; restarting to restore before database initialization")
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
}
