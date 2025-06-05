package storage

import (
	"path/filepath"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PickcodeCache 表示 pickcode 缓存的数据库模型
type PickcodeCache struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	FilePath  string    `gorm:"uniqueIndex;not null" json:"file_path"` // 文件路径作为唯一索引
	Pickcode  string    `gorm:"not null" json:"pickcode"`              // 115 pickcode
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	db     *gorm.DB
	dbOnce sync.Once
	dbErr  error
)

// InitPickcodeDB 初始化 pickcode 缓存数据库
func InitPickcodeDB() error {
	dbOnce.Do(func() {
		// 确保数据目录存在
		if err := EnsureDataDir(); err != nil {
			dbErr = err
			return
		}

		// 数据库文件路径
		dbPath := filepath.Join(DataDir, "pickcode_cache.db")

		// 打开数据库连接
		db, dbErr = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent), // 静默模式，避免过多日志
		})

		if dbErr != nil {
			return
		}

		// 自动迁移表结构
		dbErr = db.AutoMigrate(&PickcodeCache{})
	})

	return dbErr
}

// GetPickcodeFromCache 从缓存中获取 pickcode
func GetPickcodeFromCache(filePath string) (string, bool) {
	if db == nil {
		if err := InitPickcodeDB(); err != nil {
			return "", false
		}
	}

	var cache PickcodeCache
	result := db.Where("file_path = ?", filePath).First(&cache)

	if result.Error != nil {
		return "", false
	}

	return cache.Pickcode, true
}

// SavePickcodeToCache 保存 pickcode 到缓存
func SavePickcodeToCache(filePath, pickcode string) error {
	if db == nil {
		if err := InitPickcodeDB(); err != nil {
			return err
		}
	}

	cache := PickcodeCache{
		FilePath: filePath,
		Pickcode: pickcode,
	}

	// 使用 Upsert 操作，如果存在则更新，不存在则插入
	result := db.Where("file_path = ?", filePath).First(&PickcodeCache{})
	if result.Error != nil {
		// 记录不存在，插入新记录
		return db.Create(&cache).Error
	} else {
		// 记录存在，更新
		return db.Model(&PickcodeCache{}).Where("file_path = ?", filePath).Updates(map[string]interface{}{
			"pickcode":   pickcode,
			"updated_at": time.Now(),
		}).Error
	}
}

// DeletePickcodeFromCache 从缓存中删除 pickcode
func DeletePickcodeFromCache(filePath string) error {
	if db == nil {
		if err := InitPickcodeDB(); err != nil {
			return err
		}
	}

	return db.Where("file_path = ?", filePath).Delete(&PickcodeCache{}).Error
}

// ClearPickcodeCache 清空所有 pickcode 缓存
func ClearPickcodeCache() error {
	if db == nil {
		if err := InitPickcodeDB(); err != nil {
			return err
		}
	}

	return db.Exec("DELETE FROM pickcode_caches").Error
}

// GetPickcodeCacheStats 获取缓存统计信息
func GetPickcodeCacheStats() (int64, error) {
	if db == nil {
		if err := InitPickcodeDB(); err != nil {
			return 0, err
		}
	}

	var count int64
	err := db.Model(&PickcodeCache{}).Count(&count).Error
	return count, err
}
