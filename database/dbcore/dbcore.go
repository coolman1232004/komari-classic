package dbcore

import (
	"github.com/komari-monitor/komari/utils/backup"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/migrations"
	logutil "github.com/komari-monitor/komari/utils/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	instance *gorm.DB
	once     sync.Once
)

func buildSQLiteDSN(databaseFile string) string {
	if databaseFile == "" {
		databaseFile = "./data/komari.db"
	}

	params := "_busy_timeout=5000&_txlock=immediate&_journal_mode=WAL&_synchronous=NORMAL"
	separator := "?"
	if strings.Contains(databaseFile, "?") {
		separator = "&"
	}

	if strings.HasPrefix(databaseFile, "file:") {
		return databaseFile + separator + params
	}

	if databaseFile == ":memory:" {
		return "file::memory:?cache=shared&" + params
	}

	return "file:" + filepath.ToSlash(databaseFile) + separator + params
}

func GetDBInstance() *gorm.DB {
	once.Do(func() {

		var err error

		if err := backup.ApplyPending("./data", flags.DatabaseFile); err != nil {
			log.Fatalf("[restore] %v", err)
		}

		logConfig := &gorm.Config{
			Logger: logutil.NewGormLogger(),
		}

		// 根据数据库类型选择不同的连接方式
		switch flags.ApplyDatabaseTypeNormalization() {
		case flags.DatabaseTypeSQLite:
			// SQLite 连接
			// 通过 DSN 传入 _busy_timeout / _txlock 等参数，确保连接池中的每一条连接
			// 都生效：
			//   - _busy_timeout=5000：遇到写锁时最多等待 5s 再返回，避免瞬时
			//     "database is locked" 直接失败（仅靠后续 PRAGMA Exec 只作用于
			//     当时执行该语句的单条连接，池内其它连接不生效）。
			//   - _txlock=immediate：事务一开始即获取写锁，避免「先 SELECT 后写」
			//     的锁升级在并发写入下产生死锁式的立即 SQLITE_BUSY。
			//   - _journal_mode=WAL / _synchronous=NORMAL：与下方 PRAGMA 保持一致，
			//     在 DSN 层为所有连接预设。
			dsn := buildSQLiteDSN(flags.DatabaseFile)
			instance, err = gorm.Open(sqlite.Open(dsn), logConfig)
			if err != nil {
				log.Fatalf("Failed to connect to SQLite3 database: %v", err)
			}
			log.Printf("Using SQLite database file: %s", flags.DatabaseFile)
			if sqlDB, dbErr := instance.DB(); dbErr == nil {
				// SQLite 同一时刻只允许一个写者；限制连接数可避免连接池层面的写写竞争。
				// 负载历史每分钟会执行包含读和写的事务，若连接池允许多个连接，容易与
				// ping 结果等短写入撞锁并导致整批负载记录回滚。
				sqlDB.SetMaxOpenConns(1)
				sqlDB.SetMaxIdleConns(1)
				sqlDB.SetConnMaxLifetime(0)
			} else {
				log.Printf("Failed to access underlying sql.DB for SQLite tuning: %v", dbErr)
			}
			instance.Exec("PRAGMA wal = ON;")
			if err := instance.Exec("PRAGMA journal_mode = WAL;").Error; err != nil {
				log.Printf("Failed to enable WAL mode for SQLite: %v", err)
			}
			instance.Exec("PRAGMA synchronous = NORMAL;")
			instance.Exec("PRAGMA busy_timeout = 5000;")
			instance.Exec("PRAGMA cache_size = -65536;")
			instance.Exec("PRAGMA temp_store = MEMORY;")
			instance.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
		default:
			log.Fatalf("Unsupported database type: %s (supported: %s)", flags.DatabaseType, flags.SupportedDatabaseTypes())
		}
		if err := migrations.Run(migrations.Context{DB: instance}); err != nil {
			log.Fatalf("Failed to run startup migrations: %v", err)
		}
		config.SetDb(instance)
		// 自动迁移模型
		err = instance.AutoMigrate(
			&models.User{},
			&models.Client{},
			&models.Record{},
			&models.GPURecord{},
			&models.Log{},
			&models.Clipboard{},
			&models.LoadNotification{},
			&models.OfflineNotification{},
			&models.TrafficReportNotification{},
			&models.PingRecord{},
			&models.PingTask{},
			&models.OidcProvider{},
			&models.MessageSenderProvider{},
			&models.ThemeConfiguration{},
		)
		if err != nil {
			log.Fatalf("Failed to create tables: %v", err)
		}
		err = instance.Table("records_long_term").AutoMigrate(
			&models.Record{},
		)
		if err != nil {
			log.Printf("Failed to create records_long_term table, it may already exist: %v", err)
		}
		err = instance.Table("gpu_records_long_term").AutoMigrate(
			&models.GPURecord{},
		)
		if err != nil {
			log.Printf("Failed to create gpu_records_long_term table, it may already exist: %v", err)
		}
		err = instance.AutoMigrate(
			&models.Session{},
		)
		if err != nil {
			log.Printf("Failed to create Session table, it may already exist: %v", err)
		}
		err = instance.AutoMigrate(
			&models.Task{},
			&models.TaskResult{},
		)
		if err != nil {
			log.Printf("Failed to create Task and TaskResult table, it may already exist: %v", err)
		}

		// Manually create composite indexes
		if flags.IsSQLite() {
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_record_client_time ON records(client, time)")
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_record_lt_client_time ON records_long_term(client, time)")
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_gpu_record_client_time ON gpu_records(client, time)")
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_gpu_record_lt_client_time ON gpu_records_long_term(client, time)")
			instance.Exec("CREATE INDEX IF NOT EXISTS idx_ping_record_client_time ON ping_records(client, time)")
		}

	})

	return instance
}
