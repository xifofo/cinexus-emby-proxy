package storage

import (
	"time"
)

// PickcodeCache 表示 pickcode 缓存的数据库模型
type PickcodeCache struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	FilePath  string    `gorm:"uniqueIndex;not null" json:"file_path"` // 文件路径作为唯一索引
	Pickcode  string    `gorm:"not null" json:"pickcode"`              // 115 pickcode
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InitPickcodeDB 初始化 pickcode 缓存数据库
func InitPickcodeDB() error {
	// 使用统一的数据库初始化
	return InitDB()
}

// GetPickcodeFromCache 从缓存中获取 pickcode
func GetPickcodeFromCache(filePath string) (string, bool) {
	db := GetDB()
	if db == nil {
		return "", false
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
	db := GetDB()
	if db == nil {
		return InitDB()
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
	db := GetDB()
	if db == nil {
		return InitDB()
	}

	return db.Where("file_path = ?", filePath).Delete(&PickcodeCache{}).Error
}

// ClearPickcodeCache 清空所有 pickcode 缓存
func ClearPickcodeCache() error {
	db := GetDB()
	if db == nil {
		return InitDB()
	}

	return db.Exec("DELETE FROM pickcode_caches").Error
}

// GetPickcodeCacheStats 获取缓存统计信息
func GetPickcodeCacheStats() (int64, error) {
	db := GetDB()
	if db == nil {
		if err := InitDB(); err != nil {
			return 0, err
		}
	}

	var count int64
	err := db.Model(&PickcodeCache{}).Count(&count).Error
	return count, err
}
