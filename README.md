# ProxyX - MySQL Proxy with Plugin System

一个基于 go-mysql 库的 MySQL 代理，支持插件系统来扩展功能。

## 功能特性

- 🔄 透明代理 MySQL 连接
- 📝 打印所有 SQL 语句
- 🔌 插件系统，支持自定义扩展
- 📮 内置 Redis 插件，支持推送 SQL 到 Redis
- 🎯 内置过滤器插件，支持按条件过滤 SQL

## 快速开始

### 安装依赖

```bash
go mod tidy
```

### 运行

```bash
go run .
```

### 配置

在 `main.go` 中修改以下配置：

```go
proxyAddr := "127.0.0.1:4000"    // 代理监听地址
mysqlAddr := "127.0.0.1:3306"    // MySQL 服务器地址
mysqlUser := "root"              // MySQL 用户名
mysqlPassword := "123456"        // MySQL 密码
mysqlDB := ""                    // 默认数据库
```

### 连接代理

使用任意 MySQL 客户端连接到 `127.0.0.1:4000`：

```bash
mysql -h 127.0.0.1 -P 4000 -u root -p123456
```

## 插件系统

### 插件接口

```go
type Plugin interface {
    Name() string
    OnQuery(event *QueryEvent)
    OnQueryComplete(event *QueryEvent, result *mysql.Result, err error)
    Close() error
}
```

### 内置插件

#### 1. LogPlugin - 日志插件

打印 SQL 到控制台：

```go
pluginManager.Register(NewLogPlugin())
```

#### 2. RedisPlugin - Redis 插件

推送 SQL 到 Redis：

```go
redisPlugin, err := NewRedisPlugin(RedisPluginConfig{
    Addr:       "127.0.0.1:6379",
    Password:   "",
    DB:         0,
    Channel:    "mysql:queries",    // 用于 PUBLISH
    ListKey:    "mysql:query_list", // 用于 LPUSH
    MaxListLen: 1000,               // 列表最大保留1000条
    UseList:    true,               // true=LPUSH, false=PUBLISH
})
if err == nil {
    pluginManager.Register(redisPlugin)
}
```

#### 3. FilterPlugin - 过滤器插件

只处理符合条件的 SQL：

```go
pluginManager.Register(NewFilterPlugin(
    NewLogPlugin(),
    func(event *QueryEvent) bool {
        return strings.HasPrefix(strings.ToUpper(event.Query), "SELECT")
    },
))
```

### 自定义插件

实现 `Plugin` 接口即可创建自定义插件：

```go
type MyPlugin struct{}

func (p *MyPlugin) Name() string { return "MyPlugin" }

func (p *MyPlugin) OnQuery(event *QueryEvent) {
    // SQL 执行前
}

func (p *MyPlugin) OnQueryComplete(event *QueryEvent, result *mysql.Result, err error) {
    // SQL 执行后
}

func (p *MyPlugin) Close() error { return nil }
```

## QueryEvent 结构

```go
type QueryEvent struct {
    Type      string        // 事件类型: query, prepare, execute, use_db, etc.
    Query     string        // SQL语句
    Args      []interface{} // 参数（用于prepared statement）
    Database  string        // 数据库名
    Timestamp time.Time     // 时间戳
    Duration  time.Duration // 执行耗时
    Error     string        // 错误信息
    RowCount  int           // 行数
}
```

## License

MIT

