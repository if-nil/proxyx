package redisproxy

import (
	"log"
	"strings"
)

// LogPlugin Redis日志插件 - 打印命令到控制台
type LogPlugin struct{}

// NewLogPlugin 创建Redis日志插件
func NewLogPlugin() *LogPlugin {
	return &LogPlugin{}
}

func (p *LogPlugin) Name() string {
	return "RedisLogPlugin"
}

func (p *LogPlugin) OnCommand(event *CommandEvent) {
	if len(event.Args) > 0 {
		log.Printf("[Redis] %s %s", event.Command, strings.Join(event.Args, " "))
	} else {
		log.Printf("[Redis] %s", event.Command)
	}
}

func (p *LogPlugin) OnCommandComplete(event *CommandEvent) {
	if event.Error != "" {
		log.Printf("[Redis] Error: %s (duration: %v)", event.Error, event.Duration)
	} else {
		log.Printf("[Redis] OK (duration: %v)", event.Duration)
	}
}

func (p *LogPlugin) Close() error {
	return nil
}

