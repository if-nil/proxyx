package redisproxy

import "time"

// CommandEvent Redis命令事件
type CommandEvent struct {
	Command   string        `json:"command"`   // 命令名，如 GET, SET, HGET
	Args      []string      `json:"args"`      // 命令参数
	Raw       string        `json:"raw"`       // 原始命令字符串
	Timestamp time.Time     `json:"timestamp"` // 时间戳
	Duration  time.Duration `json:"duration"`  // 执行耗时
	Error     string        `json:"error"`     // 错误信息（如果有）
	Response  string        `json:"response"`  // 响应摘要
}

