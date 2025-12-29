package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-mysql-org/go-mysql/server"
	"github.com/if-nil/proxyx/config"
	"github.com/if-nil/proxyx/mysql"
	"github.com/if-nil/proxyx/redisproxy"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 启动 MySQL 代理
	if cfg.MySQL.Enabled {
		go startMySQLProxy(cfg)
	}

	// 启动 Redis 代理
	if cfg.Redis.Enabled {
		go startRedisProxy(cfg)
	}

	// 检查是否至少启用了一个代理
	if !cfg.MySQL.Enabled && !cfg.Redis.Enabled {
		log.Fatal("No proxy enabled. Please enable at least one proxy in config.")
	}

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}

func startMySQLProxy(cfg *config.Config) {
	// 创建MySQL插件管理器
	pluginManager := mysql.NewPluginManager()

	// 根据配置注册插件
	if cfg.MySQLPlugins.Log.Enabled {
		pluginManager.Register(mysql.NewLogPlugin())
	}

	if cfg.MySQLPlugins.Redis.Enabled {
		redisPlugin, err := mysql.NewRedisPlugin(cfg.MySQLPlugins.Redis)
		if err != nil {
			log.Printf("Failed to connect to Redis for MySQL plugin: %v", err)
		} else {
			pluginManager.Register(redisPlugin)
		}
	}

	defer pluginManager.Close()

	listener, err := net.Listen("tcp", cfg.MySQL.Addr)
	if err != nil {
		log.Fatalf("MySQL Proxy listen error: %v", err)
	}
	defer listener.Close()

	log.Printf("MySQL Proxy listening on %s, forwarding to %s", cfg.MySQL.Addr, cfg.MySQL.Target)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("MySQL Proxy accept error: %v", err)
			continue
		}

		go handleMySQLConnection(clientConn, cfg, pluginManager)
	}
}

func handleMySQLConnection(c net.Conn, cfg *config.Config, pluginManager *mysql.PluginManager) {
	defer c.Close()

	// 为每个客户端连接创建一个到真正MySQL的连接
	handler, err := mysql.NewHandler(
		cfg.MySQL.Target,
		cfg.MySQL.User,
		cfg.MySQL.Password,
		cfg.MySQL.Database,
		pluginManager,
	)
	if err != nil {
		log.Printf("Failed to connect to MySQL: %v", err)
		return
	}
	defer handler.Close()

	// 创建一个假的MySQL服务器连接来处理客户端请求
	conn, err := server.NewConn(c, cfg.MySQL.User, cfg.MySQL.Password, handler)
	if err != nil {
		log.Printf("Failed to create MySQL server conn: %v", err)
		return
	}

	// 持续处理客户端命令
	for {
		if err := conn.HandleCommand(); err != nil {
			log.Printf("MySQL connection closed: %v", err)
			return
		}
	}
}

func startRedisProxy(cfg *config.Config) {
	// 创建Redis插件管理器
	pluginManager := redisproxy.NewPluginManager()

	// 根据配置注册插件
	if cfg.RedisPlugins.Log.Enabled {
		pluginManager.Register(redisproxy.NewLogPlugin())
	}

	if cfg.RedisPlugins.Redis.Enabled {
		redisPlugin, err := redisproxy.NewRedisPlugin(cfg.RedisPlugins.Redis)
		if err != nil {
			log.Printf("Failed to connect to Redis for Redis proxy plugin: %v", err)
		} else {
			pluginManager.Register(redisPlugin)
		}
	}

	defer pluginManager.Close()

	// 启动Redis代理
	err := redisproxy.StartProxy(cfg.Redis.Addr, cfg.Redis.Target, pluginManager)
	if err != nil {
		log.Fatalf("Redis Proxy error: %v", err)
	}

	// 保持goroutine运行
	select {}
}
