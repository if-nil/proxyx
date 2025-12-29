package web

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/if-nil/proxyx/frontend"
	"github.com/redis/go-redis/v9"
)

// Config Web服务配置
type Config struct {
	Enabled       bool   `yaml:"enabled"`
	Addr          string `yaml:"addr"`
	RedisAddr     string `yaml:"redis_addr"`
	RedisPassword string `yaml:"redis_password"`
	RedisDB       int    `yaml:"redis_db"`
	MySQLChannel  string `yaml:"mysql_channel"`
	RedisChannel  string `yaml:"redis_channel"`
}

// Server Web服务器
type Server struct {
	config    Config
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	upgrader  websocket.Upgrader
	redis     *redis.Client
	ctx       context.Context
	cancel    context.CancelFunc
	mux       *http.ServeMux
}

// Message 推送给前端的消息
type Message struct {
	Type string          `json:"type"` // "mysql" or "redis"
	Data json.RawMessage `json:"data"`
}

// NewServer 创建Web服务器
func NewServer(config Config) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, err
	}

	// 设置默认值
	if config.MySQLChannel == "" {
		config.MySQLChannel = "mysql:queries"
	}
	if config.RedisChannel == "" {
		config.RedisChannel = "redis:commands"
	}

	return &Server{
		config:  config,
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源
			},
		},
		redis:  client,
		ctx:    ctx,
		cancel: cancel,
		mux:    http.NewServeMux(),
	}, nil
}

// Start 启动Web服务器
func (s *Server) Start() error {
	// 启动 Redis 订阅
	go s.subscribeRedis()

	// 设置 API 路由
	s.mux.HandleFunc("/ws", s.handleWebSocket)
	s.mux.HandleFunc("/api/history", s.handleHistory)

	// 设置静态文件服务（使用嵌入的文件）
	distFS, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(distFS))
	s.mux.Handle("/", s.spaHandler(fileServer, distFS))

	log.Printf("Web server listening on %s", s.config.Addr)
	return http.ListenAndServe(s.config.Addr, s.mux)
}

// spaHandler 处理SPA路由，未找到的文件返回 index.html
func (s *Server) spaHandler(fileServer http.Handler, distFS fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = path[1:] // 去掉开头的 /
		}

		// 检查文件是否存在
		_, err := fs.Stat(distFS, path)
		if err != nil {
			// 文件不存在，返回 index.html（SPA 路由支持）
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}

// subscribeRedis 订阅Redis频道
func (s *Server) subscribeRedis() {
	pubsub := s.redis.Subscribe(s.ctx, s.config.MySQLChannel, s.config.RedisChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		var msgType string
		if msg.Channel == s.config.MySQLChannel {
			msgType = "mysql"
		} else {
			msgType = "redis"
		}

		message := Message{
			Type: msgType,
			Data: json.RawMessage(msg.Payload),
		}

		s.broadcast(message)
	}
}

// broadcast 广播消息给所有客户端
func (s *Server) broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Web] Marshal error: %v", err)
		return
	}

	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for client := range s.clients {
		err := client.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Printf("[Web] Write error: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
}

// handleWebSocket 处理WebSocket连接
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Web] Upgrade error: %v", err)
		return
	}

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	log.Printf("[Web] Client connected, total: %d", len(s.clients))

	// 保持连接，读取消息（用于检测断开）
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			s.clientsMu.Lock()
			delete(s.clients, conn)
			s.clientsMu.Unlock()
			conn.Close()
			log.Printf("[Web] Client disconnected, total: %d", len(s.clients))
			break
		}
	}
}

// handleHistory 获取历史记录（从Redis List）
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 获取 MySQL 历史
	mysqlList, _ := s.redis.LRange(s.ctx, "mysql:query_list", 0, 99).Result()
	// 获取 Redis 历史
	redisList, _ := s.redis.LRange(s.ctx, "redis:command_list", 0, 99).Result()

	history := map[string][]json.RawMessage{
		"mysql": make([]json.RawMessage, 0),
		"redis": make([]json.RawMessage, 0),
	}

	for _, item := range mysqlList {
		history["mysql"] = append(history["mysql"], json.RawMessage(item))
	}
	for _, item := range redisList {
		history["redis"] = append(history["redis"], json.RawMessage(item))
	}

	json.NewEncoder(w).Encode(history)
}

// Close 关闭服务器
func (s *Server) Close() error {
	s.cancel()
	s.clientsMu.Lock()
	for client := range s.clients {
		client.Close()
	}
	s.clientsMu.Unlock()
	return s.redis.Close()
}
