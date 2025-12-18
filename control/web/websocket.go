package web

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Client WebSocket客户端
type Client struct {
	conn *websocket.Conn
	send chan []byte // 发送消息的通道
}

// WebSocketHub WebSocket中心
type WebSocketHub struct {
	// 注册的客户端
	clients map[*Client]bool

	// 注册请求
	register chan *Client

	// 注销请求
	unregister chan *Client

	// 广播消息
	broadcast chan []byte
}

// NewWebSocketHub 创建新的WebSocket中心
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
}

// ServeWebSocket 处理WebSocket连接
func (h *WebSocketHub) ServeWebSocket(c *gin.Context) {
	// 升级HTTP连接到WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket升级失败:", err)
		return
	}

	// 创建客户端
	client := &Client{
		conn: conn,
		send: make(chan []byte, 256), // 缓冲通道，避免阻塞
	}

	// 注册客户端
	h.register <- client

	// 启动写入协程（在后台发送ping心跳和消息）
	go h.writePump(client)

	// 在当前goroutine中运行readPump（阻塞直到连接关闭）
	h.readPump(client)
}

// readPump 读取消息
func (h *WebSocketHub) readPump(client *Client) {
	defer func() {
		h.unregister <- client
		close(client.send) // 关闭发送通道，通知writePump退出
		client.conn.Close()
	}()

	// 设置读取超时
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	// 处理pong响应
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		log.Printf("WebSocket收到pong响应")
		return nil
	})

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket读取失败: %v", err)
			}
			break
		}
		// 重置读取超时
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

// writePump 写入消息和ping心跳
func (h *WebSocketHub) writePump(client *Client) {
	// 设置心跳定时器（每30秒发送一次ping）
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			// 设置写入超时
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			if !ok {
				// send通道已关闭，发送关闭消息
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 发送消息
			err := client.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("WebSocket写入消息失败: %v", err)
				return
			}

		case <-ticker.C:
			// 发送ping心跳保持连接活动
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := client.conn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				log.Printf("WebSocket发送ping失败: %v", err)
				return
			}
		}
	}
}

// Start 启动WebSocket中心
func (h *WebSocketHub) Start() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("WebSocket客户端已连接，当前连接数: %d", len(h.clients))

		case client := <-h.unregister:
			delete(h.clients, client)
			log.Printf("WebSocket客户端已断开，当前连接数: %d", len(h.clients))

		case message := <-h.broadcast:
			// 将消息发送到所有客户端的send通道
			for client := range h.clients {
				select {
				case client.send <- message:
					// 消息已发送到通道
				default:
					// 通道已满，关闭客户端
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// Broadcast 广播消息
func (h *WebSocketHub) Broadcast(data interface{}) {
	jsonData, _ := json.Marshal(data)
	h.broadcast <- jsonData
}
