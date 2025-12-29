package mysql

import "github.com/go-mysql-org/go-mysql/mysql"

// FilterPlugin 过滤器插件 - 只处理符合条件的SQL
type FilterPlugin struct {
	inner     Plugin                      // 内部插件
	predicate func(event *QueryEvent) bool // 过滤条件
}

// NewFilterPlugin 创建过滤器插件
func NewFilterPlugin(inner Plugin, predicate func(event *QueryEvent) bool) *FilterPlugin {
	return &FilterPlugin{
		inner:     inner,
		predicate: predicate,
	}
}

func (p *FilterPlugin) Name() string {
	return "FilterPlugin(" + p.inner.Name() + ")"
}

func (p *FilterPlugin) OnQuery(event *QueryEvent) {
	if p.predicate(event) {
		p.inner.OnQuery(event)
	}
}

func (p *FilterPlugin) OnQueryComplete(event *QueryEvent, result *mysql.Result, err error) {
	if p.predicate(event) {
		p.inner.OnQueryComplete(event, result, err)
	}
}

func (p *FilterPlugin) Close() error {
	return p.inner.Close()
}

