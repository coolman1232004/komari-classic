package admin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/utils/backup"
	"github.com/komari-monitor/komari/web/api"
)

// copyFile 复制单个文件到目标路径（会确保父目录存在）
func copyFile(srcPath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %v", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dest.Close()

	if _, err = io.Copy(dest, src); err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}
	return nil
}

// copyDataToTempExcludingDB 将 ./data 下除了 .db/.db-wal/.db-shm 之外的所有文件复制到临时目录
func copyDataToTempExcludingDB(tempDir string) error {
	dataRoot := "./data"

	// 如果 data 目录不存在，视为无文件可复制
	if stat, err := os.Stat(dataRoot); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat data dir: %v", err)
	} else if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory", dataRoot)
	}

	return filepath.Walk(dataRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dataRoot, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup cannot include symlinks")
		}
		if backup.Reserved(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// A VACUUM snapshot replaces only our primary database. Never silently
		// discard an additional metrics database from a different version.
		name := strings.ToLower(rel)
		if name == "komari.db" || name == "komari.db-wal" || name == "komari.db-shm" || name == "komari.db-journal" {
			return nil
		}
		if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") || strings.HasSuffix(name, "-journal") {
			return fmt.Errorf("additional database files require a separate migration backup")
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("backup cannot include special files")
		}

		dst := filepath.Join(tempDir, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		return copyFile(p, dst)
	})
}

// backupSQLiteTo 使用 SQLite VACUUM INTO 将当前数据库一致性备份到指定路径
func backupSQLiteTo(destDBPath string) error {
	if err := os.MkdirAll(filepath.Dir(destDBPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory for db: %v", err)
	}

	db := dbcore.GetDBInstance()
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying database connection: %v", err)
	}

	safePath := strings.ReplaceAll(destDBPath, "'", "''")
	vacuumSQL := fmt.Sprintf("VACUUM INTO '%s'", safePath)
	if _, err = sqlDB.Exec(vacuumSQL); err != nil {
		return fmt.Errorf("sqlite VACUUM INTO failed: %v", err)
	}
	return nil
}

// DownloadBackup 用于打包 ./data 目录及数据库文件为 zip 并通过 HTTP 下载
func DownloadBackup(c *gin.Context) {
	// 1) 创建临时目录
	tempDir, err := os.MkdirTemp("", "komari-backup-*")
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, fmt.Sprintf("Error creating temporary directory: %v", err))
		return
	}
	defer os.RemoveAll(tempDir)

	// 2) 复制 ./data 下除 .db/.db-wal/.db-shm 外的所有文件到临时目录
	if err := copyDataToTempExcludingDB(tempDir); err != nil {
		api.RespondError(c, http.StatusInternalServerError, fmt.Sprintf("Error copying data to temp: %v", err))
		return
	}

	// 3) 处理数据库备份 -> 临时目录/komari.db
	destDB := filepath.Join(tempDir, "komari.db")
	dbFilePath := flags.DatabaseFile

	if flags.IsSQLite() {
		if err := backupSQLiteTo(destDB); err != nil {
			api.RespondError(c, http.StatusInternalServerError, fmt.Sprintf("Error backing up sqlite database: %v", err))
			return
		}
	} else if dbFilePath != "" {
		// 非 sqlite 的情况：若配置了文件路径且存在，则直接复制（按用户需求仍然将名称固定为 komari.db）
		if _, err := os.Stat(dbFilePath); err == nil {
			if err := copyFile(dbFilePath, destDB); err != nil {
				api.RespondError(c, http.StatusInternalServerError, fmt.Sprintf("Error copying database file: %v", err))
				return
			}
		} else if !os.IsNotExist(err) {
			api.RespondError(c, http.StatusInternalServerError, fmt.Sprintf("Error stating database file: %v", err))
			return
		}
	}

	// Complete the ZIP before sending headers, so failures cannot produce a
	// success response containing a partial archive.
	archive, err := os.CreateTemp("", "komari-download-*.zip")
	if err != nil {
		api.RespondError(c, 500, "Cannot prepare backup archive")
		return
	}
	defer os.Remove(archive.Name())
	err = backup.Write(archive, tempDir, utils.CurrentVersion)
	closeErr := archive.Close()
	if err != nil || closeErr != nil {
		api.RespondError(c, 500, "Cannot complete backup archive")
		return
	}
	checkDir, err := os.MkdirTemp("", "komari-backup-check-*")
	if err != nil {
		api.RespondError(c, 500, "Cannot validate backup archive")
		return
	}
	defer os.RemoveAll(checkDir)
	if err := backup.Stage(archive.Name(), checkDir); err != nil {
		api.RespondError(c, 500, "Backup validation failed: "+err.Error())
		return
	}
	c.FileAttachment(archive.Name(), fmt.Sprintf("backup-%d.zip", time.Now().UnixMicro()))
}
