package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/SilverSS/gameserver/types"
	"github.com/gorilla/websocket"
)

const wsServerEndpoint = "ws://eos916.asuscomm.com:9160/ws"

type GameClient struct {
	conn     *websocket.Conn
	clientID int
	username string
}

func (c *GameClient) login(ch chan<- error) {
	b, err := json.Marshal(types.Login{
		ClientID: c.clientID,
		Username: c.username,
	})
	if err != nil {
		ch <- err
		return
	}

	msg := types.WSMessage{
		Type: "login",
		Data: b,
	}

	ch <- c.conn.WriteJSON(msg)
}

func (c *GameClient) sendRandomPosition() error {
	for {
		x := float32(rand.Intn(100000)) / 1000.0
		y := float32(rand.Intn(100000)) / 1000.0
		z := float32(rand.Intn(100000)) / 1000.0

		state := types.PlayerState{
			Health:   100,
			Position: types.Vector{X: x, Y: y, Z: z},
		}
		b, err := json.Marshal(state)
		if err != nil {
			log.Fatal(err)
		}
		msg := types.WSMessage{
			Type: "playerState",
			Data: b,
		}
		if err := c.conn.WriteJSON(msg); err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Microsecond * 10)
	}
}

func newGameClient(conn *websocket.Conn, username string, c chan<- *GameClient) {
	c <- &GameClient{
		conn:     conn,
		clientID: rand.Intn(math.MaxInt),
		username: username,
	}
}

func makeGameClients(count int, mainchannel chan []*GameClient) {
	dialer := websocket.Dialer{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	clientChanel := make(chan *GameClient)
	var clients []*GameClient

	for i := 0; i < count; i++ {
		conn, _, err := dialer.Dial(wsServerEndpoint, nil)
		if err != nil {
			log.Fatal(err)
		}
		key := fmt.Sprintf("client_%d", i)
		go newGameClient(conn, key, clientChanel)
		clients = append(clients, <-clientChanel)

		time.Sleep(time.Millisecond * 10)
	}

	mainchannel <- clients
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	clientCount := 1000
	if len(os.Args) >= 2 {
		count, err := strconv.Atoi(os.Args[1])
		if err != nil {
			count = 1000
		}
		clientCount = count
	}

	mainchannel := make(chan []*GameClient)
	var clients []*GameClient

	go makeGameClients(clientCount, mainchannel)
	clients = append(clients, <-mainchannel...)

	loginCh := make(chan error)
	for _, value := range clients {
		go value.login(loginCh)
	}

	for i := 0; i < len(clients); i++ {
		err := <-loginCh
		if err != nil {
			log.Fatal(err)
		}
	}

	for _, value := range clients {
		go value.sendRandomPosition()
	}

	select {}
}
