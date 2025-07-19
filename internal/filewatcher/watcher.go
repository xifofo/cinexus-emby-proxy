package filewatcher

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cinexus/internal/config"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// FileWatcherManager 文件监控管理器，管理多个监控实例
type FileWatcherManager struct {
	watchers []*FileWatcher
	logger   *logrus.Logger
	mu       sync.RWMutex
}

// NewFileWatcherManager 创建新的文件监控管理器
func NewFileWatcherManager(configs *config.FileWatcherConfigs, logger *logrus.Logger) (*FileWatcherManager, error) {
	if !configs.Enabled {
		return nil, nil
	}

	if len(configs.Configs) == 0 {
		return nil, fmt.Errorf("文件监控已启用但没有配置任何监控项")
	}

	manager := &FileWatcherManager{
		logger:   logger,
		watchers: make([]*FileWatcher, 0, len(configs.Configs)),
	}

	// 为每个配置创建监控实例
	for i, cfg := range configs.Configs {
		watcher, err := NewFileWatcher(&cfg, logger)
		if err != nil {
			// 清理已创建的监控器
			manager.stopAll()
			return nil, fmt.Errorf("创建第%d个文件监控器失败: %w", i+1, err)
		}
		manager.watchers = append(manager.watchers, watcher)
	}

	return manager, nil
}

// Start 启动所有文件监控器
func (m *FileWatcherManager) Start() error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i, watcher := range m.watchers {
		if err := watcher.Start(); err != nil {
			// 停止已启动的监控器
			for j := 0; j < i; j++ {
				m.watchers[j].Stop()
			}
			return fmt.Errorf("启动第%d个文件监控器失败: %w", i+1, err)
		}
	}

	m.logger.Infof("文件监控管理器已启动，共启动了 %d 个监控实例", len(m.watchers))
	return nil
}

// Stop 停止所有文件监控器
func (m *FileWatcherManager) Stop() error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stopAll()
}

// stopAll 停止所有监控器（内部方法，不加锁）
func (m *FileWatcherManager) stopAll() error {
	var errors []error

	for i, watcher := range m.watchers {
		if err := watcher.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("停止第%d个文件监控器失败: %w", i+1, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("停止文件监控器时出现错误: %v", errors)
	}

	m.logger.Info("文件监控管理器已停止")
	return nil
}

// GetWatcherCount 获取监控器数量
func (m *FileWatcherManager) GetWatcherCount() int {
	if m == nil {
		return 0
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.watchers)
}

// FileWatcher 单个文件监控器
type FileWatcher struct {
	config   *config.FileWatcherConfig
	watcher  *fsnotify.Watcher
	logger   *logrus.Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
	watching bool
	mu       sync.RWMutex
}

// NewFileWatcher 创建新的文件监控器
func NewFileWatcher(cfg *config.FileWatcherConfig, log *logrus.Logger) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("创建文件监控器失败: %w", err)
	}

	fw := &FileWatcher{
		config:  cfg,
		watcher: watcher,
		logger:  log,
		stopCh:  make(chan struct{}),
	}

	return fw, nil
}

// Start 启动文件监控
func (fw *FileWatcher) Start() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.watching {
		return fmt.Errorf("文件监控器[%s]已经在运行", fw.config.Name)
	}

	// 检查源目录是否存在
	if _, err := os.Stat(fw.config.SourceDir); os.IsNotExist(err) {
		return fmt.Errorf("监控源目录不存在: %s", fw.config.SourceDir)
	}

	// 创建目标目录（如果配置允许）
	if fw.config.CreateDirs {
		if err := os.MkdirAll(fw.config.TargetDir, 0755); err != nil {
			return fmt.Errorf("创建目标目录失败: %w", err)
		}
	}

	// 添加监控目录
	if err := fw.addWatchPaths(); err != nil {
		return fmt.Errorf("添加监控路径失败: %w", err)
	}

	fw.watching = true
	fw.wg.Add(1)

	go fw.watchLoop()

	fw.logger.Infof("文件监控器[%s]已启动，监控目录: %s -> %s", fw.config.Name, fw.config.SourceDir, fw.config.TargetDir)

	// 只有在配置允许时才处理已存在的文件
	if fw.config.ProcessExistingFiles {
		// 启动后延迟处理源目录中已存在的文件，确保监控器完全就绪
		go func() {
			// 等待1秒确保监控器完全初始化
			time.Sleep(1 * time.Second)
			fw.logger.Infof("监控器[%s]开始初始扫描处理已存在的文件", fw.config.Name)
			fw.processExistingFilesInDir(fw.config.SourceDir)
		}()
	} else {
		fw.logger.Infof("监控器[%s]跳过处理已存在文件（配置已禁用）", fw.config.Name)
	}

	return nil
}

// Stop 停止文件监控
func (fw *FileWatcher) Stop() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if !fw.watching {
		return nil
	}

	close(fw.stopCh)
	fw.watcher.Close()
	fw.wg.Wait()
	fw.watching = false

	fw.logger.Infof("文件监控器[%s]已停止", fw.config.Name)
	return nil
}

// addWatchPaths 添加监控路径
func (fw *FileWatcher) addWatchPaths() error {
	// 添加根目录
	if err := fw.watcher.Add(fw.config.SourceDir); err != nil {
		return fmt.Errorf("添加根监控目录失败: %w", err)
	}

	// 如果启用递归监控，添加所有子目录
	if fw.config.Recursive {
		err := filepath.Walk(fw.config.SourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && path != fw.config.SourceDir {
				if err := fw.watcher.Add(path); err != nil {
					fw.logger.Warnf("添加子目录监控失败: %s, 错误: %v", path, err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("递归添加监控目录失败: %w", err)
		}
	}

	return nil
}

// watchLoop 监控事件循环
func (fw *FileWatcher) watchLoop() {
	defer fw.wg.Done()

	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.logger.Errorf("文件监控器[%s]错误: %v", fw.config.Name, err)

		case <-fw.stopCh:
			return
		}
	}
}

// handleEvent 处理文件系统事件
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	// 只处理创建事件
	if event.Op&fsnotify.Create == 0 {
		return
	}

	// 检查是否是目录
	info, err := os.Stat(event.Name)
	if err != nil {
		fw.logger.Warnf("获取文件信息失败: %s, 错误: %v", event.Name, err)
		return
	}

	if info.IsDir() {
		// 如果是目录且启用递归监控，添加到监控列表
		if fw.config.Recursive {
			if err := fw.watcher.Add(event.Name); err != nil {
				fw.logger.Warnf("添加新目录监控失败: %s, 错误: %v", event.Name, err)
			} else {
				fw.logger.Debugf("监控器[%s]添加新目录监控: %s", fw.config.Name, event.Name)

				fw.processExistingFilesInDir(event.Name)
			}
		}
		return
	}

	// 检查文件扩展名
	if !fw.shouldProcessFile(event.Name) {
		return
	}

	// 等待文件写入完成
	if err := fw.waitForFileReady(event.Name); err != nil {
		fw.logger.Warnf("等待文件就绪失败: %s, 错误: %v", event.Name, err)
		return
	}

	// 处理文件
	if err := fw.processFile(event.Name); err != nil {
		fw.logger.Errorf("监控器[%s]处理文件失败: %s, 错误: %v", fw.config.Name, event.Name, err)
	} else {
		fw.logger.Infof("监控器[%s]成功处理文件: %s", fw.config.Name, event.Name)
	}
}

// processExistingFilesInDir 处理目录中已存在的文件
func (fw *FileWatcher) processExistingFilesInDir(dirPath string) {
	// 异步处理，避免阻塞主监控循环
	go func() {
		fw.logger.Infof("监控器[%s]开始检查目录中已存在的文件: %s", fw.config.Name, dirPath)

		var processedCount int
		var skippedCount int
		var errorCount int

		err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fw.logger.Warnf("监控器[%s]遍历目录失败: %s, 错误: %v", fw.config.Name, path, err)
				errorCount++
				return nil // 继续处理其他文件
			}

			// 跳过目录
			if info.IsDir() {
				fw.logger.Debugf("监控器[%s]跳过目录: %s", fw.config.Name, path)
				return nil
			}

			fw.logger.Debugf("监控器[%s]检查文件: %s", fw.config.Name, path)

			// 检查文件扩展名
			if !fw.shouldProcessFile(path) {
				fw.logger.Debugf("监控器[%s]文件扩展名不匹配，跳过: %s", fw.config.Name, path)
				skippedCount++
				return nil
			}

			// 检查文件是否已经存在于目标位置
			if fw.isFileAlreadyProcessed(path) {
				fw.logger.Debugf("监控器[%s]文件已存在于目标位置，跳过: %s", fw.config.Name, path)
				skippedCount++
				return nil
			}

			fw.logger.Debugf("监控器[%s]准备处理文件: %s", fw.config.Name, path)

			// 等待文件写入完成
			if err := fw.waitForFileReady(path); err != nil {
				fw.logger.Warnf("监控器[%s]等待文件就绪失败: %s, 错误: %v", fw.config.Name, path, err)
				errorCount++
				return nil
			}

			// 处理文件
			if err := fw.processFile(path); err != nil {
				fw.logger.Errorf("监控器[%s]处理已存在文件失败: %s, 错误: %v", fw.config.Name, path, err)
				errorCount++
			} else {
				fw.logger.Infof("监控器[%s]成功处理已存在文件: %s", fw.config.Name, path)
				processedCount++
			}

			return nil
		})

		if err != nil {
			fw.logger.Errorf("监控器[%s]遍历目录失败: %s, 错误: %v", fw.config.Name, dirPath, err)
		} else {
			fw.logger.Infof("监控器[%s]完成检查目录: %s，处理了 %d 个文件，跳过 %d 个文件，%d 个错误",
				fw.config.Name, dirPath, processedCount, skippedCount, errorCount)
		}
	}()
}

// isFileAlreadyProcessed 检查文件是否已经被处理过（目标位置是否已存在）
func (fw *FileWatcher) isFileAlreadyProcessed(sourcePath string) bool {
	// 计算目标路径
	relPath, err := filepath.Rel(fw.config.SourceDir, sourcePath)
	if err != nil {
		fw.logger.Debugf("监控器[%s]计算相对路径失败: %s, 错误: %v", fw.config.Name, sourcePath, err)
		return false
	}

	targetPath := filepath.Join(fw.config.TargetDir, relPath)
	fw.logger.Debugf("监控器[%s]检查目标文件是否存在: %s", fw.config.Name, targetPath)

	// 检查目标文件是否存在
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		fw.logger.Debugf("监控器[%s]目标文件不存在: %s, 错误: %v", fw.config.Name, targetPath, err)
		return false // 目标文件不存在
	}

	// 获取源文件信息
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		fw.logger.Debugf("监控器[%s]获取源文件信息失败: %s, 错误: %v", fw.config.Name, sourcePath, err)
		return false
	}

	// 比较文件大小
	if targetInfo.Size() != sourceInfo.Size() {
		fw.logger.Debugf("监控器[%s]文件大小不匹配，需要重新处理: 源文件=%d, 目标文件=%d",
			fw.config.Name, sourceInfo.Size(), targetInfo.Size())
		return false
	}

	// 比较修改时间（允许1秒的误差）
	timeDiff := targetInfo.ModTime().Sub(sourceInfo.ModTime())
	if timeDiff < -1*time.Second || timeDiff > 1*time.Second {
		fw.logger.Debugf("监控器[%s]文件修改时间不匹配，需要重新处理: 源文件=%v, 目标文件=%v",
			fw.config.Name, sourceInfo.ModTime(), targetInfo.ModTime())
		return false
	}

	fw.logger.Debugf("监控器[%s]目标文件已存在且匹配: %s", fw.config.Name, targetPath)
	return true // 目标文件已存在且匹配
}

// shouldProcessFile 检查是否应该处理此文件
func (fw *FileWatcher) shouldProcessFile(filePath string) bool {
	// 如果没有指定扩展名，处理所有文件
	if len(fw.config.Extensions) == 0 {
		return true
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	for _, allowedExt := range fw.config.Extensions {
		if strings.ToLower(allowedExt) == ext {
			return true
		}
	}

	return false
}

// waitForFileReady 等待文件写入完成
func (fw *FileWatcher) waitForFileReady(filePath string) error {
	maxWait := 30 * time.Second
	checkInterval := 500 * time.Millisecond
	timeout := time.After(maxWait)

	var lastSize int64 = -1

	for {
		select {
		case <-timeout:
			return fmt.Errorf("等待文件就绪超时: %s", filePath)
		case <-time.After(checkInterval):
			info, err := os.Stat(filePath)
			if err != nil {
				return fmt.Errorf("获取文件信息失败: %w", err)
			}

			currentSize := info.Size()
			if currentSize == lastSize && currentSize > 0 {
				// 文件大小没有变化，认为写入完成
				return nil
			}
			lastSize = currentSize
		}
	}
}

// processFile 处理文件（复制/移动/链接）
func (fw *FileWatcher) processFile(sourcePath string) error {
	// 计算目标路径
	relPath, err := filepath.Rel(fw.config.SourceDir, sourcePath)
	if err != nil {
		return fmt.Errorf("计算相对路径失败: %w", err)
	}

	targetPath := filepath.Join(fw.config.TargetDir, relPath)

	// 创建目标目录
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 根据配置的模式处理文件
	switch fw.config.CopyMode {
	case "copy":
		return fw.copyFile(sourcePath, targetPath)
	case "move":
		return fw.moveFile(sourcePath, targetPath)
	case "link":
		return fw.linkFile(sourcePath, targetPath)
	default:
		return fw.copyFile(sourcePath, targetPath)
	}
}

// copyFile 复制文件
func (fw *FileWatcher) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}

	// 复制文件权限
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("获取源文件权限失败: %w", err)
	}

	if err := os.Chmod(dst, sourceInfo.Mode()); err != nil {
		fw.logger.Warnf("设置目标文件权限失败: %v", err)
	}

	fw.logger.Infof("监控器[%s]文件复制完成: %s -> %s", fw.config.Name, src, dst)
	return nil
}

// moveFile 移动文件
func (fw *FileWatcher) moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("移动文件失败: %w", err)
	}

	fw.logger.Infof("监控器[%s]文件移动完成: %s -> %s", fw.config.Name, src, dst)
	return nil
}

// linkFile 创建硬链接
func (fw *FileWatcher) linkFile(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		return fmt.Errorf("创建硬链接失败: %w", err)
	}

	fw.logger.Infof("监控器[%s]硬链接创建完成: %s -> %s", fw.config.Name, src, dst)
	return nil
}
