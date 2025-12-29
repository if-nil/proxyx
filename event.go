package main

import "time"

// QueryEvent 查询事件，包含SQL执行的相关信息
type QueryEvent struct {
	Type      string        `json:"type"`      // 事件类型: query, prepare, execute, use_db, etc.
	Query     string        `json:"query"`     // SQL语句
	Args      []interface{} `json:"args"`      // 参数（用于prepared statement）
	Database  string        `json:"database"`  // 数据库名
	Timestamp time.Time     `json:"timestamp"` // 时间戳
	Duration  time.Duration `json:"duration"`  // 执行耗时
	Error     string        `json:"error"`     // 错误信息（如果有）
	RowCount  int           `json:"row_count"` // 影响/返回的行数
}

