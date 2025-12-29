package main

import (
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
)

// ProxyHandler 代理Handler，将请求转发到真正的MySQL服务器
type ProxyHandler struct {
	conn          *client.Conn   // 到真正MySQL服务器的连接
	pluginManager *PluginManager // 插件管理器
	currentDB     string         // 当前数据库
}

// NewProxyHandler 创建一个新的代理Handler
func NewProxyHandler(mysqlAddr, user, password, db string, pm *PluginManager) (*ProxyHandler, error) {
	conn, err := client.Connect(mysqlAddr, user, password, db)
	if err != nil {
		return nil, err
	}
	return &ProxyHandler{
		conn:          conn,
		pluginManager: pm,
		currentDB:     db,
	}, nil
}

func (h *ProxyHandler) UseDB(dbName string) error {
	event := &QueryEvent{
		Type:      "use_db",
		Query:     dbName,
		Database:  h.currentDB,
		Timestamp: time.Now(),
	}
	h.pluginManager.OnQuery(event)

	startTime := time.Now()
	err := h.conn.UseDB(dbName)
	event.Duration = time.Since(startTime)

	if err == nil {
		h.currentDB = dbName
	}

	h.pluginManager.OnQueryComplete(event, nil, err)
	return err
}

func (h *ProxyHandler) HandleQuery(query string) (*mysql.Result, error) {
	event := &QueryEvent{
		Type:      "query",
		Query:     query,
		Database:  h.currentDB,
		Timestamp: time.Now(),
	}
	h.pluginManager.OnQuery(event)

	startTime := time.Now()
	result, err := h.conn.Execute(query)
	event.Duration = time.Since(startTime)

	h.pluginManager.OnQueryComplete(event, result, err)
	return result, err
}

func (h *ProxyHandler) HandleFieldList(table string, fieldWildcard string) ([]*mysql.Field, error) {
	event := &QueryEvent{
		Type:      "field_list",
		Query:     table + " " + fieldWildcard,
		Database:  h.currentDB,
		Timestamp: time.Now(),
	}
	h.pluginManager.OnQuery(event)

	startTime := time.Now()
	result, err := h.conn.FieldList(table, fieldWildcard)
	event.Duration = time.Since(startTime)

	h.pluginManager.OnQueryComplete(event, nil, err)
	return result, err
}

func (h *ProxyHandler) HandleStmtPrepare(query string) (int, int, interface{}, error) {
	event := &QueryEvent{
		Type:      "prepare",
		Query:     query,
		Database:  h.currentDB,
		Timestamp: time.Now(),
	}
	h.pluginManager.OnQuery(event)

	startTime := time.Now()
	stmt, err := h.conn.Prepare(query)
	event.Duration = time.Since(startTime)

	h.pluginManager.OnQueryComplete(event, nil, err)

	if err != nil {
		return 0, 0, nil, err
	}
	return stmt.ParamNum(), stmt.ColumnNum(), stmt, nil
}

func (h *ProxyHandler) HandleStmtExecute(context interface{}, query string, args []interface{}) (*mysql.Result, error) {
	event := &QueryEvent{
		Type:      "execute",
		Query:     query,
		Args:      args,
		Database:  h.currentDB,
		Timestamp: time.Now(),
	}
	h.pluginManager.OnQuery(event)

	startTime := time.Now()
	stmt := context.(*client.Stmt)
	result, err := stmt.Execute(args...)
	event.Duration = time.Since(startTime)

	h.pluginManager.OnQueryComplete(event, result, err)
	return result, err
}

func (h *ProxyHandler) HandleStmtClose(context interface{}) error {
	if stmt, ok := context.(*client.Stmt); ok {
		return stmt.Close()
	}
	return nil
}

func (h *ProxyHandler) HandleOtherCommand(cmd byte, data []byte) error {
	event := &QueryEvent{
		Type:      "other",
		Query:     string(data),
		Database:  h.currentDB,
		Timestamp: time.Now(),
	}
	h.pluginManager.OnQuery(event)
	h.pluginManager.OnQueryComplete(event, nil, nil)
	return nil
}

func (h *ProxyHandler) Close() {
	if h.conn != nil {
		h.conn.Close()
	}
}

