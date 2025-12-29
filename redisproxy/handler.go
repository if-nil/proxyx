package redisproxy

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// Handler Redis代理处理器
type Handler struct {
	targetAddr    string
	pluginManager *PluginManager
}

// NewHandler 创建Redis代理处理器
func NewHandler(targetAddr string, pm *PluginManager) *Handler {
	return &Handler{
		targetAddr:    targetAddr,
		pluginManager: pm,
	}
}

// HandleConnection 处理客户端连接
func (h *Handler) HandleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// 连接到真正的Redis服务器
	serverConn, err := net.Dial("tcp", h.targetAddr)
	if err != nil {
		log.Printf("[Redis Proxy] Failed to connect to Redis server: %v", err)
		return
	}
	defer serverConn.Close()

	// 创建带缓冲的读取器
	clientReader := bufio.NewReader(clientConn)
	serverReader := bufio.NewReader(serverConn)

	for {
		// 读取客户端命令
		command, args, raw, err := h.readCommand(clientReader)
		if err != nil {
			if err != io.EOF {
				log.Printf("[Redis Proxy] Read command error: %v", err)
			}
			return
		}

		// 创建事件
		event := &CommandEvent{
			Command:   command,
			Args:      args,
			Raw:       raw,
			Timestamp: time.Now(),
		}

		// 触发命令前事件
		h.pluginManager.OnCommand(event)

		startTime := time.Now()

		// 转发命令到Redis服务器
		_, err = serverConn.Write([]byte(raw))
		if err != nil {
			log.Printf("[Redis Proxy] Write to server error: %v", err)
			event.Error = err.Error()
			event.Duration = time.Since(startTime)
			h.pluginManager.OnCommandComplete(event)
			return
		}

		// 读取并转发响应
		response, respRaw, err := h.readResponse(serverReader)
		if err != nil {
			log.Printf("[Redis Proxy] Read response error: %v", err)
			event.Error = err.Error()
			event.Duration = time.Since(startTime)
			h.pluginManager.OnCommandComplete(event)
			return
		}

		event.Duration = time.Since(startTime)
		event.Response = response

		// 检查响应是否是错误
		if strings.HasPrefix(response, "ERR") || strings.HasPrefix(response, "WRONGTYPE") {
			event.Error = response
		}

		// 触发命令完成事件
		h.pluginManager.OnCommandComplete(event)

		// 转发响应到客户端
		_, err = clientConn.Write(respRaw)
		if err != nil {
			log.Printf("[Redis Proxy] Write to client error: %v", err)
			return
		}
	}
}

// readCommand 读取RESP协议命令
func (h *Handler) readCommand(reader *bufio.Reader) (command string, args []string, raw string, err error) {
	// 读取第一行
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", nil, "", err
	}
	raw = line

	// 检查是否是数组格式 (*N)
	if len(line) < 1 {
		return "", nil, raw, fmt.Errorf("empty line")
	}

	if line[0] == '*' {
		// RESP数组格式
		count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil {
			return "", nil, raw, err
		}

		parts := make([]string, 0, count)
		for i := 0; i < count; i++ {
			// 读取bulk string长度 ($N)
			lenLine, err := reader.ReadString('\n')
			if err != nil {
				return "", nil, raw, err
			}
			raw += lenLine

			if lenLine[0] != '$' {
				return "", nil, raw, fmt.Errorf("expected bulk string, got: %s", lenLine)
			}

			length, err := strconv.Atoi(strings.TrimSpace(lenLine[1:]))
			if err != nil {
				return "", nil, raw, err
			}

			// 读取实际内容
			data := make([]byte, length+2) // +2 for \r\n
			_, err = io.ReadFull(reader, data)
			if err != nil {
				return "", nil, raw, err
			}
			raw += string(data)

			parts = append(parts, string(data[:length]))
		}

		if len(parts) > 0 {
			command = strings.ToUpper(parts[0])
			if len(parts) > 1 {
				args = parts[1:]
			}
		}
		return command, args, raw, nil
	}

	// 内联命令格式（简单文本）
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) > 0 {
		command = strings.ToUpper(parts[0])
		if len(parts) > 1 {
			args = parts[1:]
		}
	}
	return command, args, raw, nil
}

// readResponse 读取RESP协议响应
func (h *Handler) readResponse(reader *bufio.Reader) (summary string, raw []byte, err error) {
	// 读取第一行
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", nil, err
	}
	raw = []byte(line)

	if len(line) < 1 {
		return "", raw, fmt.Errorf("empty response")
	}

	switch line[0] {
	case '+': // 简单字符串
		summary = strings.TrimSpace(line[1:])
		return summary, raw, nil

	case '-': // 错误
		summary = strings.TrimSpace(line[1:])
		return summary, raw, nil

	case ':': // 整数
		summary = strings.TrimSpace(line[1:])
		return summary, raw, nil

	case '$': // Bulk字符串
		length, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil {
			return "", raw, err
		}
		if length == -1 {
			summary = "(nil)"
			return summary, raw, nil
		}
		data := make([]byte, length+2)
		_, err = io.ReadFull(reader, data)
		if err != nil {
			return "", raw, err
		}
		raw = append(raw, data...)
		if length > 50 {
			summary = string(data[:50]) + "..."
		} else {
			summary = string(data[:length])
		}
		return summary, raw, nil

	case '*': // 数组
		count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil {
			return "", raw, err
		}
		if count == -1 {
			summary = "(nil)"
			return summary, raw, nil
		}
		// 递归读取数组元素
		for i := 0; i < count; i++ {
			_, elemRaw, err := h.readResponse(reader)
			if err != nil {
				return "", raw, err
			}
			raw = append(raw, elemRaw...)
		}
		summary = fmt.Sprintf("(%d elements)", count)
		return summary, raw, nil

	default:
		// 未知类型，尝试读取整行
		summary = strings.TrimSpace(line)
		return summary, raw, nil
	}
}

// StartProxy 启动Redis代理服务
func StartProxy(listenAddr, targetAddr string, pm *PluginManager) error {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	log.Printf("Redis Proxy listening on %s, forwarding to %s", listenAddr, targetAddr)

	handler := NewHandler(targetAddr, pm)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("[Redis Proxy] Accept error: %v", err)
				continue
			}
			go handler.HandleConnection(conn)
		}
	}()

	return nil
}

