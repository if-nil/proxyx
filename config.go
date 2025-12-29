package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Proxy   ProxyConfig   `yaml:"proxy"`
	MySQL   MySQLConfig   `yaml:"mysql"`
	Plugins PluginsConfig `yaml:"plugins"`
}

// ProxyConfig 代理服务器配置
type ProxyConfig struct {
	Addr string `yaml:"addr"` // 代理监听地址，如 "127.0.0.1:4000"
}

// MySQLConfig MySQL连接配置
type MySQLConfig struct {
	Addr     string `yaml:"addr"`     // MySQL地址，如 "127.0.0.1:3306"
	User     string `yaml:"user"`     // 用户名
	Password string `yaml:"password"` // 密码
	Database string `yaml:"database"` // 默认数据库
}

// PluginsConfig 插件配置
type PluginsConfig struct {
	Log   LogPluginConfig   `yaml:"log"`
	Redis RedisPluginConfig `yaml:"redis"`
}

// LogPluginConfig 日志插件配置
type LogPluginConfig struct {
	Enabled bool `yaml:"enabled"` // 是否启用
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 设置默认值
	config.setDefaults()

	return &config, nil
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	if c.Proxy.Addr == "" {
		c.Proxy.Addr = "127.0.0.1:4000"
	}
	if c.MySQL.Addr == "" {
		c.MySQL.Addr = "127.0.0.1:3306"
	}
	if c.MySQL.User == "" {
		c.MySQL.User = "root"
	}
	if c.Plugins.Redis.Channel == "" {
		c.Plugins.Redis.Channel = "mysql:queries"
	}
	if c.Plugins.Redis.ListKey == "" {
		c.Plugins.Redis.ListKey = "mysql:query_list"
	}
}

