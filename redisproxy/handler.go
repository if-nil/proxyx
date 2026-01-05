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
		if args == nil {
			args = []string{}
		}
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
		if strings.HasPrefix(response, "ERR") || strings.HasPrefix(response, "WRONGTYPE") ||
			strings.HasPrefix(response, "NOAUTH") || strings.HasPrefix(response, "NOPERM") {
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
	// 先peek一下第一个字节，判断是RESP格式还是内联格式
	firstByte, err := reader.Peek(1)
	if err != nil {
		return "", nil, "", err
	}

	if firstByte[0] == '*' {
		// RESP 数组格式
		return h.readRESPCommand(reader)
	}

	// 内联命令格式
	return h.readInlineCommand(reader)
}

// readRESPCommand 读取RESP格式命令
func (h *Handler) readRESPCommand(reader *bufio.Reader) (command string, args []string, raw string, err error) {
	// 读取数组长度行
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", nil, "", err
	}
	raw = line

	if len(line) < 3 || line[0] != '*' {
		return "", nil, raw, fmt.Errorf("invalid RESP array: %s", line)
	}

	count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return "", nil, raw, fmt.Errorf("invalid array count: %v", err)
	}

	if count <= 0 {
		return "", nil, raw, fmt.Errorf("empty command")
	}

	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// 读取 bulk string
		value, partRaw, err := h.readBulkString(reader)
		if err != nil {
			return "", nil, raw, err
		}
		raw += partRaw
		parts = append(parts, value)
	}

	if len(parts) > 0 {
		command = strings.ToUpper(parts[0])
		if len(parts) > 1 {
			args = parts[1:]
		}
	}
	return command, args, raw, nil
}

// readBulkString 读取 RESP Bulk String
func (h *Handler) readBulkString(reader *bufio.Reader) (value string, raw string, err error) {
	// 读取长度行
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	raw = line

	if len(line) < 3 || line[0] != '$' {
		return "", raw, fmt.Errorf("expected bulk string, got: %s", line)
	}

	length, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return "", raw, fmt.Errorf("invalid bulk string length: %v", err)
	}

	// $-1 表示 null
	if length < 0 {
		return "", raw, nil
	}

	// 读取数据 + \r\n
	data := make([]byte, length+2)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return "", raw, err
	}
	raw += string(data)

	return string(data[:length]), raw, nil
}

// readInlineCommand 读取内联命令格式
func (h *Handler) readInlineCommand(reader *bufio.Reader) (command string, args []string, raw string, err error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", nil, "", err
	}
	raw = line

	// 解析内联命令（空格分隔）
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", nil, raw, fmt.Errorf("empty command")
	}

	// 简单的空格分割（内联命令不支持带空格的参数）
	parts := strings.Fields(trimmed)
	if len(parts) > 0 {
		command = strings.ToUpper(parts[0])
		if len(parts) > 1 {
			args = parts[1:]
		}
	}
	return command, args, raw, nil
}

// readResponse 读取RESP2/RESP3协议响应
func (h *Handler) readResponse(reader *bufio.Reader) (summary string, raw []byte, err error) {
	// 读取第一个字节确定类型
	firstByte, err := reader.ReadByte()
	if err != nil {
		return "", nil, err
	}
	raw = []byte{firstByte}

	switch firstByte {
	// ============ RESP2 类型 ============
	case '+': // Simple String
		return h.readSimpleString(reader, raw)

	case '-': // Error
		return h.readSimpleString(reader, raw)

	case ':': // Integer
		return h.readSimpleString(reader, raw)

	case '$': // Bulk String
		return h.readBulkStringResponse(reader, raw)

	case '*': // Array
		return h.readArrayResponse(reader, raw)

	// ============ RESP3 新增类型 ============
	case '_': // Null (RESP3)
		return h.readNull(reader, raw)

	case ',': // Double (RESP3)
		return h.readSimpleString(reader, raw)

	case '#': // Boolean (RESP3)
		return h.readBoolean(reader, raw)

	case '!': // Blob Error (RESP3)
		return h.readBulkStringResponse(reader, raw)

	case '=': // Verbatim String (RESP3)
		return h.readVerbatimString(reader, raw)

	case '(': // Big Number (RESP3)
		return h.readSimpleString(reader, raw)

	case '%': // Map (RESP3)
		return h.readMapResponse(reader, raw)

	case '~': // Set (RESP3)
		return h.readSetResponse(reader, raw)

	case '>': // Push (RESP3)
		return h.readArrayResponse(reader, raw)

	case '|': // Attribute (RESP3)
		return h.readAttributeResponse(reader, raw)

	default:
		// 未知类型，尝试作为内联响应读取
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", raw, err
		}
		raw = append(raw, []byte(line)...)
		summary = string(firstByte) + strings.TrimSpace(line)
		return summary, raw, nil
	}
}

// readSimpleString 读取简单字符串（+、-、:、,、( 类型）
func (h *Handler) readSimpleString(reader *bufio.Reader, raw []byte) (summary string, rawBytes []byte, err error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, []byte(line)...)
	summary = strings.TrimSpace(line)
	return summary, raw, nil
}

// readNull 读取 RESP3 Null 类型
func (h *Handler) readNull(reader *bufio.Reader, raw []byte) (summary string, rawBytes []byte, err error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, []byte(line)...)
	return "(nil)", raw, nil
}

// readBoolean 读取 RESP3 Boolean 类型
func (h *Handler) readBoolean(reader *bufio.Reader, raw []byte) (summary string, rawBytes []byte, err error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, []byte(line)...)
	
	value := strings.TrimSpace(line)
	if value == "t" {
		return "true", raw, nil
	}
	return "false", raw, nil
}

// readBulkStringResponse 读取 Bulk String 响应
func (h *Handler) readBulkStringResponse(reader *bufio.Reader, raw []byte) (summary string, rawBytes []byte, err error) {
	// 读取长度
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, []byte(line)...)

	length, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return "", raw, err
	}

	// $-1 表示 null (RESP2)
	if length < 0 {
		return "(nil)", raw, nil
	}

	// 读取数据 + \r\n
	data := make([]byte, length+2)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, data...)

	// 生成摘要
	if length > 50 {
		summary = string(data[:50]) + "..."
	} else {
		summary = string(data[:length])
	}
	return summary, raw, nil
}

// readVerbatimString 读取 RESP3 Verbatim String
func (h *Handler) readVerbatimString(reader *bufio.Reader, raw []byte) (summary string, rawBytes []byte, err error) {
	// 格式: =<length>\r\n<encoding>:<data>\r\n
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, []byte(line)...)

	length, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return "", raw, err
	}

	if length < 0 {
		return "(nil)", raw, nil
	}

	// 读取数据 + \r\n
	data := make([]byte, length+2)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, data...)

	// 跳过前4个字节的编码标识（如 "txt:"）
	content := string(data[:length])
	if len(content) > 4 {
		content = content[4:]
	}

	if len(content) > 50 {
		summary = content[:50] + "..."
	} else {
		summary = content
	}
	return summary, raw, nil
}

// readArrayResponse 读取数组响应
func (h *Handler) readArrayResponse(reader *bufio.Reader, raw []byte) (summary string, rawBytes []byte, err error) {
	// 读取数组长度
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, []byte(line)...)

	count, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return "", raw, err
	}

	// *-1 表示 null 数组 (RESP2)
	if count < 0 {
		return "(nil)", raw, nil
	}

	// *0 表示空数组
	if count == 0 {
		return "(empty array)", raw, nil
	}

	// 递归读取数组元素
	for i := 0; i < count; i++ {
		_, elemRaw, err := h.readResponse(reader)
		if err != nil {
			return "", raw, err
		}
		raw = append(raw, elemRaw...)
	}

	return fmt.Sprintf("(%d elements)", count), raw, nil
}

// readMapResponse 读取 RESP3 Map 响应
func (h *Handler) readMapResponse(reader *bufio.Reader, raw []byte) (summary string, rawBytes []byte, err error) {
	// 读取 map 大小
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, []byte(line)...)

	count, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return "", raw, err
	}

	if count < 0 {
		return "(nil)", raw, nil
	}

	if count == 0 {
		return "(empty map)", raw, nil
	}

	// 读取 key-value 对（每对2个元素）
	for i := 0; i < count*2; i++ {
		_, elemRaw, err := h.readResponse(reader)
		if err != nil {
			return "", raw, err
		}
		raw = append(raw, elemRaw...)
	}

	return fmt.Sprintf("(%d entries)", count), raw, nil
}

// readSetResponse 读取 RESP3 Set 响应
func (h *Handler) readSetResponse(reader *bufio.Reader, raw []byte) (summary string, rawBytes []byte, err error) {
	// 读取 set 大小
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, []byte(line)...)

	count, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return "", raw, err
	}

	if count < 0 {
		return "(nil)", raw, nil
	}

	if count == 0 {
		return "(empty set)", raw, nil
	}

	// 读取 set 元素
	for i := 0; i < count; i++ {
		_, elemRaw, err := h.readResponse(reader)
		if err != nil {
			return "", raw, err
		}
		raw = append(raw, elemRaw...)
	}

	return fmt.Sprintf("(%d members)", count), raw, nil
}

// readAttributeResponse 读取 RESP3 Attribute 响应
func (h *Handler) readAttributeResponse(reader *bufio.Reader, raw []byte) (summary string, rawBytes []byte, err error) {
	// 读取属性数量
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, []byte(line)...)

	count, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return "", raw, err
	}

	// 读取属性 key-value 对
	for i := 0; i < count*2; i++ {
		_, elemRaw, err := h.readResponse(reader)
		if err != nil {
			return "", raw, err
		}
		raw = append(raw, elemRaw...)
	}

	// 属性后面跟着实际的响应数据
	actualSummary, actualRaw, err := h.readResponse(reader)
	if err != nil {
		return "", raw, err
	}
	raw = append(raw, actualRaw...)

	return actualSummary, raw, nil
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
