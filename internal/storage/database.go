package storage

import (
	"path/filepath"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db     *gorm.DB
	dbOnce sync.Once
	dbErr  error
)

// InitDB 初始化统一数据库
func InitDB() error {
	dbOnce.Do(func() {
		// 确保数据目录存在
		if err := EnsureDataDir(); err != nil {
			dbErr = err
			return
		}

		// 统一数据库文件路径
		dbPath := filepath.Join(DataDir, "storage.db")

		// 打开数据库连接
		db, dbErr = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent), // 静默模式，避免过多日志
		})

		if dbErr != nil {
			return
		}

		// 自动迁移所有表结构
		dbErr = db.AutoMigrate(
			&PickcodeCache{}, // pickcode 缓存表
			&MediaTask{},     // 媒体任务表
		)
	})

	return dbErr
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	if db == nil {
		if err := InitDB(); err != nil {
			return nil
		}
	}
	return db
}
