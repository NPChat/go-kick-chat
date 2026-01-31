// Package kickchat implementa um cliente WebSocket resiliente e não-oficial para a plataforma de streaming Kick.com.
//
// O foco principal desta biblioteca é estabilidade e facilidade de uso, contendo
// mecanismos internos de reconexão automática (heartbeat) e evasão de bloqueios (WAF/Cloudflare)
// através de mimetismo de User-Agent.
//
// Uso Básico
//
// O ponto de entrada principal é a função New(), que cria um cliente configurado
// para um canal específico.
//
//	client := kickchat.New("nome-do-canal")
//	client.OnMessage = func(msg kickchat.Message) {
//	    fmt.Printf("%s disse: %s", msg.User.Username, msg.Content)
//	}
//	client.Connect(context.Background())
//
// Configuração Avançada
//
// É possível customizar a chave do Pusher ou o cluster usando Functional Options:
//
//	client := kickchat.New("nome-do-canal", kickchat.WithPusherKey("CHAVE_NOVA", "us2"))
package kickchat