package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/SilverSS/gameserver/types"
	"github.com/gorilla/websocket"
)

var wsServerEndpoint string

type GameClient struct {
	conn     *websocket.Conn
	clientID int
	username string
	done     chan struct{}
}

func (c *GameClient) close() {
	if c.conn != nil {
		c.conn.Close()
	}
	close(c.done)
}

func (c *GameClient) start(ctx context.Context) {
	// 로그인 시도
	if err := c.login(); err != nil {
		log.Printf("Client %s login failed: %v", c.username, err)
		c.close()
		return
	}

	// 로그인 성공 후 위치 업데이트 시작
	c.sendRandomPosition(ctx)
}

func (c *GameClient) login() error {
	b, err := json.Marshal(types.Login{
		ClientID: c.clientID,
		Username: c.username,
	})
	if err != nil {
		return fmt.Errorf("marshal login data: %w", err)
	}

	msg := types.WSMessage{
		Type: "login",
		Data: b,
	}

	return c.conn.WriteJSON(msg)
}

func (c *GameClient) sendRandomPosition(ctx context.Context) {
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.close() // context 종료 시 명확하게 close 호출
			return
		case <-ticker.C:
			x := float32(rand.Intn(100000)) / 1000.0
			y := float32(rand.Intn(100000)) / 1000.0
			z := float32(rand.Intn(100000)) / 1000.0

			state := types.PlayerState{
				Health:   100,
				Position: types.Vector{X: x, Y: y, Z: z},
			}
			b, err := json.Marshal(state)
			if err != nil {
				select {
				case <-ctx.Done():
					c.close()
					return
				default:
					log.Printf("Client %s marshal error: %v", c.username, err)
				}
				c.close()
				return
			}
			msg := types.WSMessage{
				Type: "playerState",
				Data: b,
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				select {
				case <-ctx.Done():
					c.close()
					return
				default:
					log.Printf("Client %s write error: %v", c.username, err)
				}
				c.close()
				return
			}
		}
	}
}

func runClient(ctx context.Context, wg *sync.WaitGroup, id int) {
	defer wg.Done()

	dialer := websocket.Dialer{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, _, err := dialer.Dial(wsServerEndpoint, nil)
	if err != nil {
		log.Printf("Failed to connect client %d: %v", id, err)
		return
	}

	client := &GameClient{
		conn:     conn,
		clientID: rand.Intn(math.MaxInt),
		username: fmt.Sprintf("client_%d", id),
		done:     make(chan struct{}),
	}

	client.start(ctx)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	port := flag.String("port", "9160", "WebSocket server port")
	count := flag.Int("clients", 100000, "Number of clients")
	flag.Parse()
	//wsServerEndpoint = fmt.Sprintf("ws://eos916.asuscomm.com:%s/ws", *port)
	wsServerEndpoint = fmt.Sprintf("ws://127.0.0.1:%s/ws", *port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 종료 시그널 처리
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	clientCount := *count
	var wg sync.WaitGroup

	// 클라이언트 생성 루프를 별도 고루틴에서 실행
	go func() {
		for i := 0; i < clientCount; i++ {
			wg.Add(1)
			go runClient(ctx, &wg, i)
			time.Sleep(time.Millisecond * 10) // 연결 제한 방지를 위한 지연
		}
	}()

	// 종료 시그널 대기 및 즉시 메시지 출력
	<-sigChan
	log.Println("프로그램 종료를 시작합니다...")
	cancel()
	wg.Wait()
	log.Println("프로그램 종료 완료")
	os.Exit(0)
}
