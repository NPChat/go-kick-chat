# Go Kick Chat

Biblioteca não-oficial em Go para conectar ao chat da Kick.com via WebSocket.

![Status do Monitor](https://github.com/NPChat/go-kick-chat/actions/workflows/monitor.yml/badge.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/NPChat/go-kick-chat.svg)](https://pkg.go.dev/github.com/NPChat/go-kick-chat)

## Instalação

```bash
go get github.com/NPChat/go-kick-chat
```
## Como Usar
```go
package main

import (
	"context"
	"fmt"
	"github.com/NPChat/go-kick-chat"
)

func main() {
	// 1. Cria o cliente para o canal desejado
	client := kickchat.New("nome-do-canal")

	// 2. Define o callback de mensagens
	client.OnMessage = func(msg kickchat.Message) {
		fmt.Printf("[%s] %s: %s\n", msg.Channel, msg.User.Username, msg.Content)
	}

	// 3. Define o callback de erros (opcional, mas recomendado)
	client.OnError = func(err error) {
		fmt.Printf("Erro de conexão: %v\n", err)
	}

	// 4. Conecta (Bloqueante)
	// Use uma goroutine se precisar fazer outras coisas
	fmt.Println("Conectando ao chat...")
	if err := client.Connect(context.Background()); err != nil {
		panic(err)
	}
}

```

## Configuração Avançada (Nova Chave ou Proxy)
Se a Kick mudar a chave do Pusher ou você precisar de um Client HTTP customizado:

```go
import (
    "net/http"
    "time"
    "github.com/NPChat/go-kick-chat"
)

func main() {
    // Exemplo: Configurando chave manual e timeout customizado
    client := kickchat.New("xqc", 
        kickchat.WithPusherKey("NOVA_CHAVE_AQUI", "us2"),
        kickchat.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
    )
    
    // ...
}
```

## Monitoramento e Manutenção
Se o badge no topo estiver Verde, a lib está funcionando. Se estiver Vermelho, já fomos notificados e estamos arrumando.