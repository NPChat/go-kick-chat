package kickchat

import "time"

// Message representa uma mensagem de chat recebida
type Message struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Channel   string    `json:"channel"` 
	User      User      `json:"sender"`
	CreatedAt time.Time `json:"created_at"`
}

// User representa o remetente de uma mensagem
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Slug     string `json:"slug"`
}