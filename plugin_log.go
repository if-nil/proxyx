package main

import (
	"log"

	"github.com/go-mysql-org/go-mysql/mysql"
)

// LogPlugin 日志插件 - 打印SQL到控制台
type LogPlugin struct{}

// NewLogPlugin 创建日志插件
func NewLogPlugin() *LogPlugin {
	return &LogPlugin{}
}

func (p *LogPlugin) Name() string {
	return "LogPlugin"
}

func (p *LogPlugin) OnQuery(event *QueryEvent) {
	log.Printf("[SQL] [%s] %s", event.Type, event.Query)
	if len(event.Args) > 0 {
		log.Printf("[SQL] Args: %v", event.Args)
	}
}

func (p *LogPlugin) OnQueryComplete(event *QueryEvent, result *mysql.Result, err error) {
	if err != nil {
		log.Printf("[SQL] Error: %v (duration: %v)", err, event.Duration)
	} else {
		log.Printf("[SQL] Success (duration: %v, rows: %d)", event.Duration, event.RowCount)
	}
}

func (p *LogPlugin) Close() error {
	return nil
}

