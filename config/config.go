package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"Puff/storage"
)

// Config 应用配置结构
type Config struct {
	Server   ServerConfig   `json:"server"`
	SMTP     SMTPConfig     `json:"smtp"`
	Telegram TelegramConfig `json:"telegram"`
	Monitor  MonitorConfig  `json:"monitor"`
	Log      LogConfig      `json:"log"`
}

// ServerConfig 服务器配置结构
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

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// 设置默认值
	setDefaults(cfg)

	// 从数据库加载配置（如缺失则回填默认值）
	if err := loadFromDatabase(cfg); err != nil {
		return nil, fmt.Errorf("加载数据库配置失败: %v", err)
	}

	return cfg, nil
}

// setDefaults 设置默认配置值
func setDefaults(cfg *Config) {
	cfg.Server.Port = "8080"
	cfg.Server.Username = "puff"
	cfg.Server.Password = "puff123"

	cfg.Monitor.CheckInterval = 5 * time.Minute
	cfg.Monitor.ConcurrentLimit = 50
	cfg.Monitor.Timeout = 30 * time.Second
	cfg.Monitor.CacheDuration = 1 * time.Hour

	cfg.Log.Level = "info"
	cfg.Log.File = ""
}

// loadFromDatabase 从SQLite加载配置（并在缺失时写入默认值）
func loadFromDatabase(cfg *Config) error {
	settings, err := storage.GetAllSettings()
	if err != nil {
		return err
	}

	// 如果缺失则写入默认值
	if err := backfillDefaults(cfg, settings); err != nil {
		return err
	}

	// 重新读取（确保含默认值）
	settings, err = storage.GetAllSettings()
	if err != nil {
		return err
	}

	applySetting := func(key string, apply func(string)) {
		if v, ok := settings[key]; ok {
			apply(strings.TrimSpace(v))
		}
	}

	applySetting("server_port", func(v string) { cfg.Server.Port = v })
	applySetting("server_username", func(v string) { cfg.Server.Username = v })
	applySetting("server_password", func(v string) { cfg.Server.Password = v })

	applySetting("smtp_host", func(v string) { cfg.SMTP.Host = v })
	applySetting("smtp_port", func(v string) {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SMTP.Port = n
		}
	})
	applySetting("smtp_user", func(v string) { cfg.SMTP.User = v })
	applySetting("smtp_pass", func(v string) { cfg.SMTP.Password = v })
	applySetting("smtp_from", func(v string) { cfg.SMTP.From = v })
	applySetting("smtp_to", func(v string) { cfg.SMTP.To = v })
	applySetting("smtp_enabled", func(v string) { cfg.SMTP.Enabled = parseBool(v) })

	applySetting("telegram_bot_token", func(v string) { cfg.Telegram.BotToken = v })
	applySetting("telegram_chat_id", func(v string) { cfg.Telegram.ChatID = v })
	applySetting("telegram_enabled", func(v string) { cfg.Telegram.Enabled = parseBool(v) })

	applySetting("monitor_check_interval", func(v string) {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Monitor.CheckInterval = time.Duration(n) * time.Second
		}
	})
	applySetting("monitor_concurrent_limit", func(v string) {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Monitor.ConcurrentLimit = n
		}
	})
	applySetting("monitor_timeout", func(v string) {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Monitor.Timeout = time.Duration(n) * time.Second
		}
	})
	applySetting("monitor_cache_duration", func(v string) {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Monitor.CacheDuration = time.Duration(n) * time.Second
		}
	})

	applySetting("log_level", func(v string) { cfg.Log.Level = v })

	// 如果存在空值，回落到默认并写回数据库
	if err := ensureDefaultsIfEmpty(cfg, settings); err != nil {
		return err
	}

	return nil
}

// backfillDefaults 将缺失的键写入数据库
func backfillDefaults(cfg *Config, settings map[string]string) error {
	defaults := map[string]string{
		"server_port":              cfg.Server.Port,
		"server_username":          cfg.Server.Username,
		"server_password":          cfg.Server.Password,
		"smtp_host":                cfg.SMTP.Host,
		"smtp_port":                fmt.Sprintf("%d", cfg.SMTP.Port),
		"smtp_user":                cfg.SMTP.User,
		"smtp_pass":                cfg.SMTP.Password,
		"smtp_from":                cfg.SMTP.From,
		"smtp_to":                  cfg.SMTP.To,
		"smtp_enabled":             fmt.Sprintf("%t", cfg.SMTP.Enabled),
		"telegram_bot_token":       cfg.Telegram.BotToken,
		"telegram_chat_id":         cfg.Telegram.ChatID,
		"telegram_enabled":         fmt.Sprintf("%t", cfg.Telegram.Enabled),
		"monitor_check_interval":   fmt.Sprintf("%d", int(cfg.Monitor.CheckInterval.Seconds())),
		"monitor_concurrent_limit": fmt.Sprintf("%d", cfg.Monitor.ConcurrentLimit),
		"monitor_timeout":          fmt.Sprintf("%d", int(cfg.Monitor.Timeout.Seconds())),
		"monitor_cache_duration":   fmt.Sprintf("%d", int(cfg.Monitor.CacheDuration.Seconds())),
		"log_level":                cfg.Log.Level,
	}

	missing := map[string]string{}
	for k, v := range defaults {
		if _, ok := settings[k]; !ok {
			missing[k] = v
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return storage.UpsertSettings(missing)
}

// ensureDefaultsIfEmpty 将空值重置为默认值并持久化
func ensureDefaultsIfEmpty(cfg *Config, settings map[string]string) error {
	updates := map[string]string{}

	if cfg.Server.Username == "" {
		cfg.Server.Username = "puff"
		updates["server_username"] = cfg.Server.Username
	}
	if cfg.Server.Password == "" {
		cfg.Server.Password = "puff123"
		updates["server_password"] = cfg.Server.Password
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
		updates["server_port"] = cfg.Server.Port
	}

	if cfg.SMTP.Port == 0 {
		cfg.SMTP.Port = 587
		updates["smtp_port"] = fmt.Sprintf("%d", cfg.SMTP.Port)
	}
	if cfg.Monitor.CheckInterval == 0 {
		cfg.Monitor.CheckInterval = 5 * time.Minute
		updates["monitor_check_interval"] = fmt.Sprintf("%d", int(cfg.Monitor.CheckInterval.Seconds()))
	}
	if cfg.Monitor.ConcurrentLimit == 0 {
		cfg.Monitor.ConcurrentLimit = 50
		updates["monitor_concurrent_limit"] = fmt.Sprintf("%d", cfg.Monitor.ConcurrentLimit)
	}
	if cfg.Monitor.Timeout == 0 {
		cfg.Monitor.Timeout = 30 * time.Second
		updates["monitor_timeout"] = fmt.Sprintf("%d", int(cfg.Monitor.Timeout.Seconds()))
	}
	if cfg.Monitor.CacheDuration == 0 {
		cfg.Monitor.CacheDuration = 1 * time.Hour
		updates["monitor_cache_duration"] = fmt.Sprintf("%d", int(cfg.Monitor.CacheDuration.Seconds()))
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
		updates["log_level"] = cfg.Log.Level
	}

	if len(updates) == 0 {
		return nil
	}
	return storage.UpsertSettings(updates)
}

// parseBool 将字符串转换为布尔值
func parseBool(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Validate 验证配置
func (cfg *Config) Validate() error {
	if cfg.Server.Username == "" {
		return fmt.Errorf("服务器用户名不能为空")
	}
	if cfg.Server.Password == "" {
		return fmt.Errorf("服务器密码不能为空")
	}

	if cfg.Monitor.CheckInterval < 5*time.Second {
		return fmt.Errorf("检查间隔不能小于5秒")
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
