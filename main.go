package main

import (
	"log"
	"net"

	"github.com/go-mysql-org/go-mysql/server"
)

func main() {
	proxyAddr := "127.0.0.1:4000"
	mysqlAddr := "127.0.0.1:3306"
	mysqlUser := "root"
	mysqlPassword := "123456"
	mysqlDB := ""

	// 创建插件管理器
	pluginManager := NewPluginManager()

	// 注册日志插件（打印SQL到控制台）
	pluginManager.Register(NewLogPlugin())

	// 注册 Redis 插件（推送SQL到Redis）
	// 取消下面的注释来启用 Redis 插件
	/*
		redisPlugin, err := NewRedisPlugin(RedisPluginConfig{
			Addr:       "127.0.0.1:6379",
			Password:   "",
			DB:         0,
			Channel:    "mysql:queries",    // 用于 PUBLISH
			ListKey:    "mysql:query_list", // 用于 LPUSH
			MaxListLen: 1000,               // 列表最大保留1000条
			UseList:    true,               // 使用 LPUSH 模式
		})
		if err != nil {
			log.Printf("Failed to connect to Redis: %v", err)
		} else {
			pluginManager.Register(redisPlugin)
		}
	*/

	// 你也可以使用过滤器插件，只处理特定的SQL
	// 例如：只记录 SELECT 语句
	/*
		pluginManager.Register(NewFilterPlugin(
			NewLogPlugin(),
			func(event *QueryEvent) bool {
				return strings.HasPrefix(strings.ToUpper(event.Query), "SELECT")
			},
		))
	*/

	defer pluginManager.Close()

	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Printf("MySQL Proxy listening on %s, forwarding to %s", proxyAddr, mysqlAddr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		go func(c net.Conn) {
			defer c.Close()

			// 为每个客户端连接创建一个到真正MySQL的连接
			handler, err := NewProxyHandler(mysqlAddr, mysqlUser, mysqlPassword, mysqlDB, pluginManager)
			if err != nil {
				log.Printf("Failed to connect to MySQL: %v", err)
				return
			}
			defer handler.Close()

			// 创建一个假的MySQL服务器连接来处理客户端请求
			conn, err := server.NewConn(c, mysqlUser, mysqlPassword, handler)
			if err != nil {
				log.Printf("Failed to create server conn: %v", err)
				return
			}

			// 持续处理客户端命令
			for {
				if err := conn.HandleCommand(); err != nil {
					log.Printf("Connection closed: %v", err)
					return
				}
			}
		}(clientConn)
	}
}
