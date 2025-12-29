package config

import (
	"os"

	"github.com/if-nil/proxyx/mysql"
	"github.com/if-nil/proxyx/redisproxy"
	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	MySQL        MySQLProxyConfig   `yaml:"mysql_proxy"`
	Redis        RedisProxyConfig   `yaml:"redis_proxy"`
	MySQLPlugins MySQLPluginsConfig `yaml:"mysql_plugins"`
	RedisPlugins RedisPluginsConfig `yaml:"redis_plugins"`
}

// MySQLProxyConfig MySQL代理配置
type MySQLProxyConfig struct {
	Enabled  bool   `yaml:"enabled"`  // 是否启用MySQL代理
	Addr     string `yaml:"addr"`     // 代理监听地址
	Target   string `yaml:"target"`   // MySQL服务器地址
	User     string `yaml:"user"`     // 用户名
	Password string `yaml:"password"` // 密码
	Database string `yaml:"database"` // 默认数据库
}

// RedisProxyConfig Redis代理配置
type RedisProxyConfig struct {
	Enabled bool   `yaml:"enabled"` // 是否启用Redis代理
	Addr    string `yaml:"addr"`    // 代理监听地址
	Target  string `yaml:"target"`  // Redis服务器地址
}

// MySQLPluginsConfig MySQL插件配置
type MySQLPluginsConfig struct {
	Log   LogPluginConfig         `yaml:"log"`
	Redis mysql.RedisPluginConfig `yaml:"redis"`
}

// RedisPluginsConfig Redis代理插件配置
type RedisPluginsConfig struct {
	Log   LogPluginConfig              `yaml:"log"`
	Redis redisproxy.RedisPluginConfig `yaml:"redis"`
}

// LogPluginConfig 日志插件配置
type LogPluginConfig struct {
	Enabled bool `yaml:"enabled"` // 是否启用
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
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
	// MySQL代理默认值
	if c.MySQL.Addr == "" {
		c.MySQL.Addr = "127.0.0.1:4000"
	}
	if c.MySQL.Target == "" {
		c.MySQL.Target = "127.0.0.1:3306"
	}
	if c.MySQL.User == "" {
		c.MySQL.User = "root"
	}

	// Redis代理默认值
	if c.Redis.Addr == "" {
		c.Redis.Addr = "127.0.0.1:6400"
	}
	if c.Redis.Target == "" {
		c.Redis.Target = "127.0.0.1:6379"
	}

	// MySQL插件默认值
	if c.MySQLPlugins.Redis.Channel == "" {
		c.MySQLPlugins.Redis.Channel = "mysql:queries"
	}
	if c.MySQLPlugins.Redis.ListKey == "" {
		c.MySQLPlugins.Redis.ListKey = "mysql:query_list"
	}

	// Redis插件默认值
	if c.RedisPlugins.Redis.Channel == "" {
		c.RedisPlugins.Redis.Channel = "redis:commands"
	}
	if c.RedisPlugins.Redis.ListKey == "" {
		c.RedisPlugins.Redis.ListKey = "redis:command_list"
	}
}
