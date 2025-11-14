package web

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Client WebSocket客户端
type Client struct {
	conn *websocket.Conn
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
		clients:   make(map[*Client]bool),
		register:  make(chan *Client),
		unregister: make(chan *Client),
		broadcast: make(chan []byte),
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
	defer conn.Close()

	// 创建客户端
	client := &Client{conn: conn}

	// 注册客户端
	h.register <- client

	// 启动读取协程
	go h.readPump(client)

	// 启动写入协程
	go h.writePump(client)
}

// readPump 读取消息
func (h *WebSocketHub) readPump(client *Client) {
	defer func() {
		h.unregister <- client
		client.conn.Close()
	}()

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket读取失败:", err)
			break
		}
	}
}

// writePump 写入消息
func (h *WebSocketHub) writePump(client *Client) {
	defer client.conn.Close()

	for {
		select {
		case message := <-h.broadcast:
			err := client.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
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
			for client := range h.clients {
				err := client.conn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					client.conn.Close()
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