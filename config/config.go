package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置结构
type Config struct {
	Server   ServerConfig   `json:"server"`
	SMTP     SMTPConfig     `json:"smtp"`
	Telegram TelegramConfig `json:"telegram"`
	Monitor  MonitorConfig  `json:"monitor"`
	Log      LogConfig      `json:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// SMTPConfig 邮件配置
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
	Enabled  bool   `json:"enabled"`
}

// TelegramConfig Telegram配置
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
	Enabled  bool   `json:"enabled"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	CheckInterval   time.Duration `json:"check_interval"`   // 检查间隔
	ConcurrentLimit int           `json:"concurrent_limit"` // 并发限制
	Timeout         time.Duration `json:"timeout"`          // 查询超时
	CacheDuration   time.Duration `json:"cache_duration"`   // 缓存时间
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `json:"level"`
	File  string `json:"file"`
}

// DomainList 域名列表结构
type DomainList struct {
	Domains []string `yaml:"domains"`
}

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// 设置默认值
	setDefaults(cfg)

	// 从环境变量加载配置
	if err := loadFromEnv(cfg); err != nil {
		return nil, fmt.Errorf("加载环境变量失败: %v", err)
	}

	return cfg, nil
}

// LoadDomains 加载域名列表
func LoadDomains(filename string) ([]string, error) {
	if filename == "" {
		filename = "domains.yml"
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取域名列表文件失败: %v", err)
	}

	var domainList DomainList
	if err := yaml.Unmarshal(data, &domainList); err != nil {
		return nil, fmt.Errorf("解析域名列表文件失败: %v", err)
	}

	// 过滤空域名
	domains := make([]string, 0, len(domainList.Domains))
	for _, domain := range domainList.Domains {
		domain = strings.TrimSpace(domain)
		if domain != "" {
			domains = append(domains, domain)
		}
	}

	return domains, nil
}

// setDefaults 设置默认配置值
func setDefaults(cfg *Config) {
	cfg.Server.Port = "8080"
	cfg.Server.Username = "admin"
	cfg.Server.Password = "admin123"

	cfg.Monitor.CheckInterval = 5 * time.Minute
	cfg.Monitor.ConcurrentLimit = 50
	cfg.Monitor.Timeout = 30 * time.Second
	cfg.Monitor.CacheDuration = 1 * time.Hour

	cfg.Log.Level = "info"
	cfg.Log.File = "domain-monitor.log"
}

// loadFromEnv 从环境变量加载配置
func loadFromEnv(cfg *Config) error {
	// 服务器配置
	if port := os.Getenv("PORT"); port != "" {
		cfg.Server.Port = port
	}
	if username := os.Getenv("APP_USERNAME"); username != "" {
		cfg.Server.Username = username
	}
	if password := os.Getenv("PASSWORD"); password != "" {
		cfg.Server.Password = password
	}

	// SMTP配置
	cfg.SMTP.Host = os.Getenv("SMTP_HOST")
	cfg.SMTP.User = os.Getenv("SMTP_USER")
	cfg.SMTP.Password = os.Getenv("SMTP_PASS")
	cfg.SMTP.From = os.Getenv("SMTP_FROM")
	cfg.SMTP.To = os.Getenv("SMTP_TO")
	cfg.SMTP.Enabled = cfg.SMTP.Host != "" && cfg.SMTP.User != ""

	if portStr := os.Getenv("SMTP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.SMTP.Port = port
		} else {
			cfg.SMTP.Port = 587 // 默认值
		}
	} else {
		cfg.SMTP.Port = 587
	}

	// Telegram配置
	cfg.Telegram.BotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	cfg.Telegram.ChatID = os.Getenv("TELEGRAM_CHAT_ID")
	cfg.Telegram.Enabled = cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != ""

	// 监控配置
	if intervalStr := os.Getenv("CHECK_INTERVAL"); intervalStr != "" {
		if interval, err := strconv.Atoi(intervalStr); err == nil {
			cfg.Monitor.CheckInterval = time.Duration(interval) * time.Second
		}
	}

	if limitStr := os.Getenv("CONCURRENT_LIMIT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			cfg.Monitor.ConcurrentLimit = limit
		}
	}

	if timeoutStr := os.Getenv("TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			cfg.Monitor.Timeout = time.Duration(timeout) * time.Second
		}
	}

	if cacheStr := os.Getenv("CACHE_DURATION"); cacheStr != "" {
		if cache, err := strconv.Atoi(cacheStr); err == nil {
			cfg.Monitor.CacheDuration = time.Duration(cache) * time.Second
		}
	}

	// 日志配置
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Log.Level = level
	}
	if file := os.Getenv("LOG_FILE"); file != "" {
		cfg.Log.File = file
	}

	return nil
}

// Validate 验证配置
func (cfg *Config) Validate() error {
	if cfg.Server.Username == "" {
		return fmt.Errorf("服务器用户名不能为空")
	}
	if cfg.Server.Password == "" {
		return fmt.Errorf("服务器密码不能为空")
	}

	if cfg.Monitor.CheckInterval < time.Minute {
		return fmt.Errorf("检查间隔不能小于1分钟")
	}

	if cfg.Monitor.ConcurrentLimit <= 0 {
		return fmt.Errorf("并发限制必须大于0")
	}

	if cfg.Monitor.Timeout <= 0 {
		return fmt.Errorf("查询超时时间必须大于0")
	}

	return nil
}

// GetNotificationEnabled 获取通知是否启用
func (cfg *Config) GetNotificationEnabled() bool {
	return cfg.SMTP.Enabled || cfg.Telegram.Enabled
}
