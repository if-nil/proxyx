package redisproxy

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

// RedisPluginConfig Redis插件配置
type RedisPluginConfig struct {
	Enabled    bool   `yaml:"enabled"`      // 是否启用
	Addr       string `yaml:"addr"`         // Redis地址，如 "127.0.0.1:6379"
	Password   string `yaml:"password"`     // Redis密码
	DB         int    `yaml:"db"`           // Redis数据库
	Channel    string `yaml:"channel"`      // 发布的频道名
	ListKey    string `yaml:"list_key"`     // 列表键名（用于LPUSH）
	MaxListLen int64  `yaml:"max_list_len"` // 列表最大长度（0表示不限制）
	UseList    bool   `yaml:"use_list"`     // true: 使用LPUSH, false: 使用PUBLISH
}

// RedisPlugin Redis插件 - 推送命令到Redis
type RedisPlugin struct {
	client *redis.Client
	config RedisPluginConfig
	ctx    context.Context
}

// NewRedisPlugin 创建Redis插件
func NewRedisPlugin(config RedisPluginConfig) (*RedisPlugin, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	ctx := context.Background()

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	// 设置默认值
	if config.Channel == "" {
		config.Channel = "redis:commands"
	}
	if config.ListKey == "" {
		config.ListKey = "redis:command_list"
	}

	return &RedisPlugin{
		client: client,
		config: config,
		ctx:    ctx,
	}, nil
}

func (p *RedisPlugin) Name() string {
	return "RedisPlugin"
}

func (p *RedisPlugin) OnCommand(event *CommandEvent) {
	// 命令开始时不做处理，等待完成
}

func (p *RedisPlugin) OnCommandComplete(event *CommandEvent) {
	data, jsonErr := json.Marshal(event)
	if jsonErr != nil {
		log.Printf("[Redis RedisPlugin] JSON marshal error: %v", jsonErr)
		return
	}

	if p.config.UseList {
		// 使用 LPUSH 推送到列表
		if err := p.client.LPush(p.ctx, p.config.ListKey, data).Err(); err != nil {
			log.Printf("[Redis RedisPlugin] LPUSH error: %v", err)
		}
		// 如果设置了最大长度，进行裁剪
		if p.config.MaxListLen > 0 {
			p.client.LTrim(p.ctx, p.config.ListKey, 0, p.config.MaxListLen-1)
		}
	} else {
		// 使用 PUBLISH 发布到频道
		if err := p.client.Publish(p.ctx, p.config.Channel, data).Err(); err != nil {
			log.Printf("[Redis RedisPlugin] PUBLISH error: %v", err)
		}
	}
}

func (p *RedisPlugin) Close() error {
	return p.client.Close()
}

