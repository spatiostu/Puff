package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileStorage 文件存储
type FileStorage struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStorage 创建新的文件存储
func NewFileStorage(basePath string) *FileStorage {
	// 确保目录存在
	os.MkdirAll(basePath, 0755)

	return &FileStorage{
		basePath: basePath,
	}
}

// SaveDomainInfo 保存域名信息
func (fs *FileStorage) SaveDomainInfo(domain string, info interface{}) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	filename := filepath.Join(fs.basePath, fmt.Sprintf("%s.json", domain))

	// 创建文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	// 编码为JSON并写入
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(info); err != nil {
		return fmt.Errorf("编码JSON失败: %v", err)
	}

	return nil
}

// LoadDomainInfo 加载域名信息
func (fs *FileStorage) LoadDomainInfo(domain string, info interface{}) error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	filename := filepath.Join(fs.basePath, fmt.Sprintf("%s.json", domain))

	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", filename)
	}

	// 打开文件
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 解码JSON
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(info); err != nil {
		return fmt.Errorf("解码JSON失败: %v", err)
	}

	return nil
}

// DeleteDomainInfo 删除域名信息
func (fs *FileStorage) DeleteDomainInfo(domain string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	filename := filepath.Join(fs.basePath, fmt.Sprintf("%s.json", domain))

	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除文件失败: %v", err)
	}

	return nil
}

// ListDomains 列出所有域名
func (fs *FileStorage) ListDomains() ([]string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %v", err)
	}

	var domains []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			domain := name[:len(name)-5] // 移除.json扩展名
			domains = append(domains, domain)
		}
	}

	return domains, nil
}

// SaveHistory 保存历史记录
func (fs *FileStorage) SaveHistory(domain string, events []interface{}) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	historyDir := filepath.Join(fs.basePath, "history")
	os.MkdirAll(historyDir, 0755)

	filename := filepath.Join(historyDir, fmt.Sprintf("%s_history.json", domain))

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建历史文件失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(events); err != nil {
		return fmt.Errorf("编码历史JSON失败: %v", err)
	}

	return nil
}

// LoadHistory 加载历史记录
func (fs *FileStorage) LoadHistory(domain string) ([]interface{}, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	historyDir := filepath.Join(fs.basePath, "history")
	filename := filepath.Join(historyDir, fmt.Sprintf("%s_history.json", domain))

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return []interface{}{}, nil // 返回空历史
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开历史文件失败: %v", err)
	}
	defer file.Close()

	var events []interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&events); err != nil {
		return nil, fmt.Errorf("解码历史JSON失败: %v", err)
	}

	return events, nil
}

// AppendHistory 追加历史记录
func (fs *FileStorage) AppendHistory(domain string, event interface{}) error {
	// 先加载现有历史
	history, err := fs.LoadHistory(domain)
	if err != nil {
		return err
	}

	// 追加新事件
	history = append(history, event)

	// 限制历史记录数量（保留最近的1000条）
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}

	// 保存更新后的历史
	return fs.SaveHistory(domain, history)
}

// ExportData 导出所有数据
func (fs *FileStorage) ExportData(outputPath string) error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// 创建输出文件
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建导出文件失败: %v", err)
	}
	defer file.Close()

	// 获取所有域名
	domains, err := fs.ListDomains()
	if err != nil {
		return err
	}

	exportData := make(map[string]interface{})
	exportData["export_time"] = time.Now()
	exportData["domains"] = make(map[string]interface{})

	// 导出每个域名的数据
	for _, domain := range domains {
		domainData := make(map[string]interface{})

		// 加载域名信息
		var info map[string]interface{}
		if err := fs.LoadDomainInfo(domain, &info); err == nil {
			domainData["info"] = info
		}

		// 加载历史记录
		if history, err := fs.LoadHistory(domain); err == nil {
			domainData["history"] = history
		}

		exportData["domains"].(map[string]interface{})[domain] = domainData
	}

	// 编码为JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(exportData); err != nil {
		return fmt.Errorf("编码导出JSON失败: %v", err)
	}

	return nil
}

// ImportData 导入数据
func (fs *FileStorage) ImportData(inputPath string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// 打开导入文件
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("打开导入文件失败: %v", err)
	}
	defer file.Close()

	// 解码JSON
	var importData map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&importData); err != nil {
		return fmt.Errorf("解码导入JSON失败: %v", err)
	}

	// 检查数据格式
	domains, ok := importData["domains"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("无效的导入数据格式")
	}

	// 导入每个域名的数据
	for domain, domainData := range domains {
		data, ok := domainData.(map[string]interface{})
		if !ok {
			continue
		}

		// 导入域名信息
		if info, exists := data["info"]; exists {
			if err := fs.SaveDomainInfo(domain, info); err != nil {
				return fmt.Errorf("导入域名 %s 信息失败: %v", domain, err)
			}
		}

		// 导入历史记录
		if historyData, exists := data["history"]; exists {
			if history, ok := historyData.([]interface{}); ok {
				if err := fs.SaveHistory(domain, history); err != nil {
					return fmt.Errorf("导入域名 %s 历史失败: %v", domain, err)
				}
			}
		}
	}

	return nil
}

// Cleanup 清理过期数据
func (fs *FileStorage) Cleanup(maxAge time.Duration) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return fmt.Errorf("读取目录失败: %v", err)
	}

	removedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			filename := filepath.Join(fs.basePath, entry.Name())
			if err := os.Remove(filename); err == nil {
				removedCount++
			}
		}
	}

	fmt.Printf("清理了 %d 个过期文件\n", removedCount)
	return nil
}

// GetStorageStats 获取存储统计信息
func (fs *FileStorage) GetStorageStats() (map[string]interface{}, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	stats := make(map[string]interface{})

	// 统计文件数量和大小
	var totalSize int64
	var fileCount int

	err := filepath.Walk(fs.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("统计存储信息失败: %v", err)
	}

	stats["total_size"] = totalSize
	stats["file_count"] = fileCount
	stats["base_path"] = fs.basePath

	return stats, nil
}
