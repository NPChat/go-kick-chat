package kickchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultPusherKey     = "32cbd69e4b950bf97679"
	defaultPusherCluster = "us2"
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
)

// Client gerencia a conexão com o Kick e a API
type Client struct {
	channel       string
	chatroomID    int
	pusherKey     string
	pusherCluster string

	OnMessage func(msg Message)
	OnError   func(err error)

	conn       *websocket.Conn
	mu         sync.Mutex
	httpClient *http.Client
}

type Option func(*Client)

// WithPusherKey permite sobrescrever a chave padrão (caso a Kick mude)
func WithPusherKey(key, cluster string) Option {
	return func(c *Client) {
		c.pusherKey = key
		c.pusherCluster = cluster
	}
}

// WithHTTPClient permite injetar um cliente HTTP customizado
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// New cria um novo cliente
func New(channel string, opts ...Option) *Client {
	c := &Client{
		channel:       channel,
		pusherKey:     defaultPusherKey,
		pusherCluster: defaultPusherCluster,
		OnMessage:     func(m Message) {},
		OnError:       func(e error) {},
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Connect inicia a conexão e mantém o loop de reconexão
func (c *Client) Connect(ctx context.Context) error {
	if err := c.resolveChannelID(); err != nil {
		return fmt.Errorf("falha inicial ao resolver canal '%s': %w", c.channel, err)
	}

	backoff := 5 * time.Second

	for {
		if ctx.Err() != nil {
			return nil
		}

		if err := c.connectWS(ctx); err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}

			c.OnError(fmt.Errorf("conexão caiu ou falhou: %v. Reconectando em %v...", err, backoff))

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
				_ = c.resolveChannelID()
				continue
			}
		}
	}
}

// resolveChannelID obtém o ID do chatroom via API
func (c *Client) resolveChannelID() error {
	u := fmt.Sprintf("https://kick.com/api/v1/channels/%s", c.channel)

	req, _ := http.NewRequest("GET", u, nil)

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9") 
	req.Header.Set("Referer", "https://kick.com/")      
	req.Header.Set("Origin", "https://kick.com")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API retornou status %d (possível bloqueio Cloudflare ou canal inexistente)", resp.StatusCode)
	}

	var data struct {
		Chatroom struct {
			ID int `json:"id"`
		} `json:"chatroom"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("erro ao decodificar JSON: %w", err)
	}

	if data.Chatroom.ID == 0 {
		return errors.New("chatroom ID não encontrado na resposta")
	}

	c.chatroomID = data.Chatroom.ID
	return nil
}

// connectWS estabelece a conexão WebSocket com o Pusher
func (c *Client) connectWS(ctx context.Context) error {
	u := url.URL{
		Scheme: "wss",
		Host:   fmt.Sprintf("ws-%s.pusher.com", c.pusherCluster),
		Path:   "/app/" + c.pusherKey,
	}
	q := u.Query()
	q.Set("protocol", "7")
	q.Set("client", "js")
	q.Set("version", "8.4.0")
	q.Set("flash", "false")
	u.RawQuery = q.Encode()

	headers := http.Header{}
	headers.Add("User-Agent", userAgent)

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	go func() {
		<-ctx.Done()
		c.closeConn()
	}()

	defer c.closeConn()

	wsCtx, cancelWs := context.WithCancel(ctx)
	defer cancelWs()

	// Handshake Pusher
	subMsg := fmt.Sprintf(`{"event":"pusher:subscribe","data":{"auth":"","channel":"chatrooms.%d.v2"}}`, c.chatroomID)
	if err := c.safeWrite([]byte(subMsg)); err != nil {
		return err
	}

	// Heartbeat roda isolado
	go c.heartbeat(wsCtx)

	for {
		_, msgBytes, err := c.conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		c.handleMessage(msgBytes)
	}
}

// handleMessage processa mensagens recebidas do WebSocket
func (c *Client) handleMessage(msgBytes []byte) {
	type pusherEvent struct {
		Event string `json:"event"`
		Data  string `json:"data"`
	}

	var event pusherEvent
	if err := json.Unmarshal(msgBytes, &event); err != nil {
		return
	}

	switch event.Event {
	case "pusher:connection_established":
		return // Conexão OK
	case "pusher:error":
		c.OnError(fmt.Errorf("ERRO PUSHER (chave inválida?): %s", event.Data))
		c.closeConn() // Força reconexão
	case "App\\Events\\ChatMessageEvent":
		var chatMsg Message
		if err := json.Unmarshal([]byte(event.Data), &chatMsg); err == nil {
			chatMsg.Channel = c.channel
			c.OnMessage(chatMsg)
		}
	}
}

// heartbeat envia pings periódicos para manter a conexão viva
func (c *Client) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ping := []byte(`{"event":"pusher:ping","data":{}}`)
			if err := c.safeWrite(ping); err != nil {
				return
			}
		}
	}
}

func (c *Client) safeWrite(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return errors.New("conexão fechada")
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// closeConn fecha a conexão WebSocket de forma segura
func (c *Client) closeConn() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}