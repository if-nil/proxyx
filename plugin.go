package main

import (
	"log"

	"github.com/go-mysql-org/go-mysql/mysql"
)

// Plugin 插件接口
type Plugin interface {
	// Name 返回插件名称
	Name() string

	// OnQuery 当执行查询时调用（在执行前）
	OnQuery(event *QueryEvent)

	// OnQueryComplete 当查询完成时调用（在执行后）
	OnQueryComplete(event *QueryEvent, result *mysql.Result, err error)

	// Close 关闭插件，释放资源
	Close() error
}

// PluginManager 插件管理器
type PluginManager struct {
	plugins []Plugin
}

// NewPluginManager 创建插件管理器
func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make([]Plugin, 0),
	}
}

// Register 注册插件
func (pm *PluginManager) Register(p Plugin) {
	pm.plugins = append(pm.plugins, p)
	log.Printf("[PluginManager] Registered plugin: %s", p.Name())
}

// OnQuery 触发所有插件的 OnQuery
func (pm *PluginManager) OnQuery(event *QueryEvent) {
	for _, p := range pm.plugins {
		p.OnQuery(event)
	}
}

// OnQueryComplete 触发所有插件的 OnQueryComplete
func (pm *PluginManager) OnQueryComplete(event *QueryEvent, result *mysql.Result, err error) {
	for _, p := range pm.plugins {
		p.OnQueryComplete(event, result, err)
	}
}

// Close 关闭所有插件
func (pm *PluginManager) Close() error {
	for _, p := range pm.plugins {
		if err := p.Close(); err != nil {
			log.Printf("[PluginManager] Error closing plugin %s: %v", p.Name(), err)
		}
	}
	return nil
}

