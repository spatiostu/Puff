package web

import (
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// DomainList 域名列表结构
type DomainList struct {
	Domains []string `yaml:"domains"`
}

// addDomainToConfig 添加域名到配置文件
func (s *Server) addDomainToConfig(domain string) error {
	domainList, err := s.loadDomainConfig()
	if err != nil {
		return err
	}

	// 检查域名是否已存在
	for _, existingDomain := range domainList.Domains {
		if strings.EqualFold(existingDomain, domain) {
			return fmt.Errorf("domain already exists")
		}
	}

	// 添加新域名
	domainList.Domains = append(domainList.Domains, domain)

	return s.saveDomainConfig(domainList)
}

// removeDomainFromConfig 从配置文件删除域名
func (s *Server) removeDomainFromConfig(domain string) error {
	domainList, err := s.loadDomainConfig()
	if err != nil {
		return err
	}

	// 查找并删除域名
	found := false
	newDomains := make([]string, 0, len(domainList.Domains))
	for _, existingDomain := range domainList.Domains {
		if !strings.EqualFold(existingDomain, domain) {
			newDomains = append(newDomains, existingDomain)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("domain not found")
	}

	domainList.Domains = newDomains
	return s.saveDomainConfig(domainList)
}

// loadDomainConfig 加载域名配置
func (s *Server) loadDomainConfig() (*DomainList, error) {
	file, err := os.Open("domains.yml")
	if err != nil {
		// 如果文件不存在，创建一个空的配置
		if os.IsNotExist(err) {
			return &DomainList{Domains: []string{}}, nil
		}
		return nil, fmt.Errorf("failed to open domains.yml: %v", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read domains.yml: %v", err)
	}

	var domainList DomainList
	if err := yaml.Unmarshal(data, &domainList); err != nil {
		return nil, fmt.Errorf("failed to parse domains.yml: %v", err)
	}

	return &domainList, nil
}

// saveDomainConfig 保存域名配置
func (s *Server) saveDomainConfig(domainList *DomainList) error {
	data, err := yaml.Marshal(domainList)
	if err != nil {
		return fmt.Errorf("failed to marshal domain config: %v", err)
	}

	// 备份原文件
	if _, err := os.Stat("domains.yml"); err == nil {
		if err := s.backupFile("domains.yml"); err != nil {
			return fmt.Errorf("failed to backup domains.yml: %v", err)
		}
	}

	// 写入新配置
	if err := os.WriteFile("domains.yml", data, 0644); err != nil {
		return fmt.Errorf("failed to write domains.yml: %v", err)
	}

	return nil
}

// backupFile 备份文件
func (s *Server) backupFile(filename string) error {
	src, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(filename + ".backup")
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}
