package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// WhoisServer WHOIS服务器配置
type WhoisServer struct {
	Server string `json:"server"` // 服务器地址
	Port   int    `json:"port"`   // 端口
}

// RDAPServer RDAP服务器配置
type RDAPServer struct {
	Server string `json:"server"` // 服务器地址
}

// TLDServers TLD服务器配置
type TLDServers struct {
	Whois WhoisServer `json:"whois"`
	RDAP  RDAPServer  `json:"rdap"`
}

// DetectionPatterns 检测模式配置
type DetectionPatterns struct {
	AvailablePatterns     []string `json:"available_patterns"`
	RedemptionPatterns    []string `json:"redemption_patterns"`
	PendingDeletePatterns []string `json:"pending_delete_patterns"`
	ExpiredPatterns       []string `json:"expired_patterns"`
	HoldPatterns          []string `json:"hold_patterns"`
	TransferLockPatterns  []string `json:"transfer_lock_patterns"`
	RegisteredPatterns    []string `json:"registered_patterns"`
}

var (
	serversConfig  map[string]TLDServers
	patternsConfig DetectionPatterns
	configsLoaded  bool
	configMutex    sync.RWMutex
)

// LoadServerConfigs 加载服务器配置
func LoadServerConfigs() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if configsLoaded {
		return nil
	}

	// 加载服务器配置
	serversFile, err := os.ReadFile("config/servers.json")
	if err != nil {
		return fmt.Errorf("读取服务器配置文件失败: %v", err)
	}

	if err := json.Unmarshal(serversFile, &serversConfig); err != nil {
		return fmt.Errorf("解析服务器配置失败: %v", err)
	}

	// 加载检测模式配置
	patternsFile, err := os.ReadFile("config/detection_patterns.json")
	if err != nil {
		return fmt.Errorf("读取检测模式配置文件失败: %v", err)
	}

	if err := json.Unmarshal(patternsFile, &patternsConfig); err != nil {
		return fmt.Errorf("解析检测模式配置失败: %v", err)
	}

	configsLoaded = true
	return nil
}

// GetWhoisServerByTLD 根据TLD获取WHOIS服务器
func GetWhoisServerByTLD(tld string) (WhoisServer, bool) {
	if err := LoadServerConfigs(); err != nil {
		return WhoisServer{}, false
	}

	configMutex.RLock()
	defer configMutex.RUnlock()

	server, exists := serversConfig[tld]
	if !exists {
		return WhoisServer{}, false
	}

	return server.Whois, true
}

// GetRDAPServerByTLD 根据TLD获取RDAP服务器
func GetRDAPServerByTLD(tld string) (RDAPServer, bool) {
	if err := LoadServerConfigs(); err != nil {
		return RDAPServer{}, false
	}

	configMutex.RLock()
	defer configMutex.RUnlock()

	server, exists := serversConfig[tld]
	if !exists {
		return RDAPServer{}, false
	}

	return server.RDAP, true
}

// GetDetectionPatterns 获取检测模式
func GetDetectionPatterns() DetectionPatterns {
	LoadServerConfigs() // 确保配置已加载

	configMutex.RLock()
	defer configMutex.RUnlock()

	return patternsConfig
}

// GetSupportedTLDs 获取支持的TLD列表
func GetSupportedTLDs() []string {
	if err := LoadServerConfigs(); err != nil {
		return []string{}
	}

	configMutex.RLock()
	defer configMutex.RUnlock()

	tlds := make([]string, 0, len(serversConfig))
	for tld := range serversConfig {
		tlds = append(tlds, tld)
	}

	return tlds
}

// AddOrUpdateTLD 添加或更新TLD配置
func AddOrUpdateTLD(tld string, whoisServer, rdapServer string, whoisPort int) error {
	if err := LoadServerConfigs(); err != nil {
		return err
	}

	serversConfig[tld] = TLDServers{
		Whois: WhoisServer{
			Server: whoisServer,
			Port:   whoisPort,
		},
		RDAP: RDAPServer{
			Server: rdapServer,
		},
	}

	// 保存到文件
	return SaveServerConfigs()
}

// SaveServerConfigs 保存服务器配置到文件
func SaveServerConfigs() error {
	data, err := json.MarshalIndent(serversConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化服务器配置失败: %v", err)
	}

	return os.WriteFile("config/servers.json", data, 0644)
}
