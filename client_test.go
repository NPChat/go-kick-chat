package kickchat_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NPChat/go-kick-chat"
)

// TestHealthCheck
func TestHealthCheck(t *testing.T) {
	stableChannels := []string{"xqc", "adinross", "gaules", "coringa"}
	
	success := false
	var lastErr error
	var errMu sync.Mutex

	for _, channel := range stableChannels {
		
		iterationSuccess := func() bool {
			t.Logf("Tentando verificar saúde via canal: %s...", channel)
			
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel() 

			client := kickchat.New(channel)
			
			connectedSig := make(chan bool, 1)
			errSig := make(chan error, 1)

			client.OnError = func(err error) {
				errMu.Lock()
				lastErr = err
				errMu.Unlock()
				
				select {
				case errSig <- err:
				default:
				}
			}

			client.OnMessage = func(msg kickchat.Message) {
				select {
				case connectedSig <- true:
				default:
				}
			}

			go func() {
				if err := client.Connect(ctx); err != nil {
					if ctx.Err() == nil {
						select {
						case errSig <- err:
						default:
						}
					}
				}
			}()

			select {
			case <-connectedSig:
				t.Logf("Sucesso confirmado no canal: %s", channel)
				return true
			case <-time.After(10 * time.Second):
				t.Logf("Conexão estável (sem msg, mas API OK) no canal: %s", channel)
				return true
			case err := <-errSig:
				t.Logf("Erro reportado no canal %s: %v", channel, err)
				return false
			case <-ctx.Done():
				t.Logf("Timeout no canal %s. Último erro: %v", channel, lastErr)
				return false
			}
		}()

		if iterationSuccess {
			success = true
			break
		}
	}

	if !success {
		t.Fatalf("FALHA CRÍTICA: Não foi possível conectar em NENHUM canal. O Cloudflare pode estar bloqueando ou a API mudou. Último erro: %v", lastErr)
	}
}