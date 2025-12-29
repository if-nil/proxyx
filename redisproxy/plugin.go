package redisproxy

import "log"

// Plugin Redis代理插件接口
type Plugin interface {
	// Name 返回插件名称
	Name() string

	// OnCommand 当执行命令时调用（在执行前）
	OnCommand(event *CommandEvent)

	// OnCommandComplete 当命令完成时调用（在执行后）
	OnCommandComplete(event *CommandEvent)

	// Close 关闭插件，释放资源
	Close() error
}

// PluginManager Redis插件管理器
type PluginManager struct {
	plugins []Plugin
}

// NewPluginManager 创建Redis插件管理器
func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make([]Plugin, 0),
	}
}

// Register 注册插件
func (pm *PluginManager) Register(p Plugin) {
	pm.plugins = append(pm.plugins, p)
	log.Printf("[Redis PluginManager] Registered plugin: %s", p.Name())
}

// OnCommand 触发所有插件的 OnCommand
func (pm *PluginManager) OnCommand(event *CommandEvent) {
	for _, p := range pm.plugins {
		p.OnCommand(event)
	}
}

// OnCommandComplete 触发所有插件的 OnCommandComplete
func (pm *PluginManager) OnCommandComplete(event *CommandEvent) {
	for _, p := range pm.plugins {
		p.OnCommandComplete(event)
	}
}

// Close 关闭所有插件
func (pm *PluginManager) Close() error {
	for _, p := range pm.plugins {
		if err := p.Close(); err != nil {
			log.Printf("[Redis PluginManager] Error closing plugin %s: %v", p.Name(), err)
		}
	}
	return nil
}

