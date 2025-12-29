package main

import (
	"flag"
	"log"
	"net"

	"github.com/go-mysql-org/go-mysql/server"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 创建插件管理器
	pluginManager := NewPluginManager()

	// 根据配置注册插件
	if config.Plugins.Log.Enabled {
		pluginManager.Register(NewLogPlugin())
	}

	if config.Plugins.Redis.Enabled {
		redisPlugin, err := NewRedisPlugin(config.Plugins.Redis)
		if err != nil {
			log.Printf("Failed to connect to Redis: %v", err)
		} else {
			pluginManager.Register(redisPlugin)
		}
	}

	defer pluginManager.Close()

	listener, err := net.Listen("tcp", config.Proxy.Addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Printf("MySQL Proxy listening on %s, forwarding to %s", config.Proxy.Addr, config.MySQL.Addr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		go handleConnection(clientConn, config, pluginManager)
	}
}

func handleConnection(c net.Conn, config *Config, pluginManager *PluginManager) {
	defer c.Close()

	// 为每个客户端连接创建一个到真正MySQL的连接
	handler, err := NewProxyHandler(
		config.MySQL.Addr,
		config.MySQL.User,
		config.MySQL.Password,
		config.MySQL.Database,
		pluginManager,
	)
	if err != nil {
		log.Printf("Failed to connect to MySQL: %v", err)
		return
	}
	defer handler.Close()

	// 创建一个假的MySQL服务器连接来处理客户端请求
	conn, err := server.NewConn(c, config.MySQL.User, config.MySQL.Password, handler)
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
}
