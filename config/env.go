package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// CreateDefaultEnvFile 创建默认的.env文件，使用UTF-8编码
func CreateDefaultEnvFile() error {
	envFile := ".env"

	// 检查文件是否已存在
	if _, err := os.Stat(envFile); err == nil {
		return nil // 文件已存在，不需要创建
	}

	defaultContent := `# 服务器配置
PORT=8080
USERNAME=admin
PASSWORD=admin123

# 邮件通知配置
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your_email@gmail.com
SMTP_PASS=your_app_password
SMTP_FROM=your_email@gmail.com
SMTP_TO=notify@example.com
SMTP_ENABLED=false

# Telegram通知配置
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_CHAT_ID=your_chat_id
TELEGRAM_ENABLED=false

# 监控配置
CHECK_INTERVAL=300
CONCURRENT_LIMIT=50
TIMEOUT=30
`

	// 确保内容是有效的UTF-8
	if !utf8.ValidString(defaultContent) {
		return fmt.Errorf("默认内容不是有效的UTF-8编码")
	}

	// 以UTF-8编码创建文件
	file, err := os.Create(envFile)
	if err != nil {
		return fmt.Errorf("无法创建.env文件: %v", err)
	}
	defer file.Close()

	// 写入UTF-8 BOM (可选，但有助于Windows识别编码)
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("写入BOM失败: %v", err)
	}

	// 以UTF-8编码写入内容
	if _, err := file.WriteString(defaultContent); err != nil {
		return fmt.Errorf("写入默认内容失败: %v", err)
	}

	return nil
}

// UpdateEnvFileSimple 简单但可靠的更新.env文件，使用UTF-8编码
func UpdateEnvFileSimple(updates map[string]string) error {
	envFile := ".env"

	// 如果文件不存在，创建默认文件
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		if err := CreateDefaultEnvFile(); err != nil {
			return err
		}
	}

	// 读取现有文件内容，确保正确处理UTF-8编码
	content := make(map[string]string)
	comments := make(map[int]string) // 存储注释和空行
	orderedKeys := []string{}

	// 读取文件内容到内存，处理可能的编码问题
	fileData, err := os.ReadFile(envFile)
	if err != nil {
		return fmt.Errorf("无法读取.env文件: %v", err)
	}

	// 检查和处理BOM
	fileContent := string(fileData)
	if len(fileData) >= 3 && fileData[0] == 0xEF && fileData[1] == 0xBB && fileData[2] == 0xBF {
		// 存在UTF-8 BOM，跳过前3个字节
		fileContent = string(fileData[3:])
	}

	// 确保文件内容是有效的UTF-8
	if !utf8.ValidString(fileContent) {
		return fmt.Errorf(".env文件不是有效的UTF-8编码")
	}

	// 逐行解析
	scanner := bufio.NewScanner(strings.NewReader(fileContent))
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// 处理注释和空行
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			comments[lineNum] = line
			continue
		}

		// 处理配置项
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				content[key] = value
				orderedKeys = append(orderedKeys, key)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("解析文件内容失败: %v", err)
	}

	// 更新配置项
	for key, value := range updates {
		content[key] = value
		// 如果是新的配置项，添加到相应位置
		found := false
		for _, existingKey := range orderedKeys {
			if existingKey == key {
				found = true
				break
			}
		}
		if !found {
			orderedKeys = append(orderedKeys, key)
		}
	}

	// 创建临时文件，确保UTF-8编码
	tempFile := envFile + ".tmp"
	tempFileHandle, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("无法创建临时文件: %v", err)
	}
	defer tempFileHandle.Close()

	// 写入UTF-8 BOM以确保Windows正确识别编码
	if _, err := tempFileHandle.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("写入BOM失败: %v", err)
	}

	// 按标准格式写入文件
	sections := []struct {
		comment string
		keys    []string
	}{
		{"# 服务器配置", []string{"PORT", "USERNAME", "PASSWORD"}},
		{"# 邮件通知配置", []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASS", "SMTP_FROM", "SMTP_TO", "SMTP_ENABLED"}},
		{"# Telegram通知配置", []string{"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID", "TELEGRAM_ENABLED"}},
		{"# 监控配置", []string{"CHECK_INTERVAL", "CONCURRENT_LIMIT", "TIMEOUT"}},
	}

	writer := bufio.NewWriter(tempFileHandle)

	for i, section := range sections {
		if i > 0 {
			writer.WriteString("\n")
		}
		writer.WriteString(section.comment + "\n")

		for _, key := range section.keys {
			if value, exists := content[key]; exists {
				writer.WriteString(fmt.Sprintf("%s=%s\n", key, value))
			}
		}
	}

	// 写入其他未分类的配置项
	hasOthers := false
	for _, key := range orderedKeys {
		found := false
		for _, section := range sections {
			for _, sectionKey := range section.keys {
				if sectionKey == key {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			if !hasOthers {
				writer.WriteString("\n# 其他配置\n")
				hasOthers = true
			}
			writer.WriteString(fmt.Sprintf("%s=%s\n", key, content[key]))
		}
	}

	// 确保写入的内容是有效的UTF-8
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	// 验证写入的文件内容
	tempFileHandle.Seek(0, 0)
	verifyData, err := io.ReadAll(tempFileHandle)
	if err == nil && !utf8.Valid(verifyData[3:]) { // 跳过BOM验证UTF-8
		os.Remove(tempFile)
		return fmt.Errorf("写入的文件内容不是有效的UTF-8编码")
	}

	// 原子性替换文件
	if err := os.Rename(tempFile, envFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("替换文件失败: %v", err)
	}

	return nil
}

// LoadEnvFile 加载.env文件，使用UTF-8编码
func LoadEnvFile() error {
	envFile := ".env"

	// 检查文件是否存在
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		// 文件不存在是正常的
		return nil
	}

	// 读取文件内容，确保正确处理UTF-8编码
	fileData, err := os.ReadFile(envFile)
	if err != nil {
		return fmt.Errorf("无法读取.env文件: %v", err)
	}

	// 检查和处理BOM
	fileContent := string(fileData)
	if len(fileData) >= 3 && fileData[0] == 0xEF && fileData[1] == 0xBB && fileData[2] == 0xBF {
		// 存在UTF-8 BOM，跳过前3个字节
		fileContent = string(fileData[3:])
	}

	// 确保文件内容是有效的UTF-8
	if !utf8.ValidString(fileContent) {
		return fmt.Errorf(".env文件不是有效的UTF-8编码")
	}

	// 逐行解析
	scanner := bufio.NewScanner(strings.NewReader(fileContent))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析键值对
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// 设置环境变量（如果还没有设置的话）
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}

	return scanner.Err()
}

// ensureUTF8 确保字符串是有效的UTF-8编码
func ensureUTF8(data []byte) (string, error) {
	// 检查UTF-8 BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		// 跳过BOM，使用UTF-8内容
		content := string(data[3:])
		if utf8.ValidString(content) {
			return content, nil
		}
	}

	// 直接尝试UTF-8
	content := string(data)
	if utf8.ValidString(content) {
		return content, nil
	}

	// 如果不是有效的UTF-8，尝试从GBK转换（Windows中文环境常见）
	// 注意：这里简化处理，实际项目中可能需要引入编码检测库
	return "", fmt.Errorf("文件编码不是UTF-8，请确保.env文件使用UTF-8编码保存")
}

// writeUTF8File 以UTF-8编码写入文件
func writeUTF8File(filename string, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入UTF-8 BOM
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}

	// 确保内容是有效的UTF-8
	if !utf8.ValidString(content) {
		return fmt.Errorf("内容不是有效的UTF-8编码")
	}

	// 写入内容
	_, err = file.WriteString(content)
	return err
}
