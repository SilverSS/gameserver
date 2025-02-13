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

func (c *GameClient) start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer c.close()

	// 로그인 시도
	if err := c.login(); err != nil {
		log.Printf("Client %s login failed: %v", c.username, err)
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
	ticker := time.NewTicker(time.Microsecond * 50)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
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
				log.Printf("Client %s marshal error: %v", c.username, err)
				return
			}
			msg := types.WSMessage{
				Type: "playerState",
				Data: b,
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				log.Printf("Client %s write error: %v", c.username, err)
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

	wg.Add(1)
	go client.start(ctx, wg)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	port := flag.String("port", "9160", "WebSocket server port")
	count := flag.Int("clients", 1000, "Number of clients")
	flag.Parse()
	wsServerEndpoint = fmt.Sprintf("ws://Localhost:%s/ws", *port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 종료 시그널 처리
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	clientCount := *count

	var wg sync.WaitGroup

	// 클라이언트 생성 및 실행
	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go runClient(ctx, &wg, i)
		time.Sleep(time.Millisecond * 10) // 연결 제한 방지를 위한 지연
	}

	// 종료 시그널 대기
	<-sigChan
	log.Println("Shutting down...")
	cancel()
	wg.Wait()
}
