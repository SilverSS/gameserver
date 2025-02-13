package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"runtime"

	"github.com/SilverSS/gameserver/types"
	"github.com/anthdm/hollywood/actor"
	"github.com/gorilla/websocket"
)

type PlayerState struct {
	Position types.Vector
	Health   int
	Velocity types.Vector
}

type PlayerSession struct {
	sessionID int
	clientID  int
	username  string
	inLobby   bool
	conn      *websocket.Conn
}

// Receive implements actor.Receiver.
func (s *PlayerSession) Receive(c *actor.Context) {
	switch c.Message().(type) {
	case actor.Started:
		s.readLoop()
		//c.SpawnChild(newPlayerState, "playerState")
	}
}

func (s *PlayerSession) readLoop() {
	var msg types.WSMessage
	for {
		if err := s.conn.ReadJSON(&msg); err != nil {
			fmt.Println("read error", err)
			return
		}
		go s.handleMessage(msg)
	}
}

func (s *PlayerSession) handleMessage(msg types.WSMessage) {
	switch msg.Type {
	case "login":
		var loginMsg types.Login
		if err := json.Unmarshal(msg.Data, &loginMsg); err != nil {
			panic(err)
		}
		s.clientID = loginMsg.ClientID
		s.username = loginMsg.Username
	case "playerState":
		var ps types.PlayerState
		if err := json.Unmarshal(msg.Data, &ps); err != nil {
			panic(err)
		}
		fmt.Println(ps)
	}
}

func newPlayerSession(sid int, conn *websocket.Conn) actor.Producer {
	return func() actor.Receiver {
		return &PlayerSession{
			conn:      conn,
			sessionID: sid,
		}
	}
}

type GameServer struct {
	ctx      *actor.Context
	sessions map[*actor.PID]struct{}
}

func newGameServer() actor.Receiver {
	return &GameServer{
		sessions: make(map[*actor.PID]struct{}),
	}
}

func (s *GameServer) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		s.startHTTP()
		s.ctx = c
		_ = msg
	}
}

func (s *GameServer) startHTTP() {
	fmt.Printf("starting HTTP server on port %s\n", port)
	go func() {
		http.HandleFunc("/ws", s.handleWS)
		strPort := fmt.Sprintf(":%s", port)
		http.ListenAndServe(strPort, nil)
	}()
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// handles the upgrade of the websocket
func (s *GameServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("ws upgrade err: ", err)
		return
	}

	fmt.Println("new client is trying to connect")
	sid := rand.Intn(math.MaxInt)
	pid := s.ctx.SpawnChild(newPlayerSession(sid, conn), fmt.Sprintf("playersession_%d", sid))
	s.sessions[pid] = struct{}{}
	fmt.Printf("client with sid %d and pid %s just connected\n", sid, pid)
}

var port string

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	portFlag := flag.String("port", "9160", "<portNumber>")
	flag.Parse()
	port = *portFlag

	e, _ := actor.NewEngine(actor.NewEngineConfig())
	e.Spawn(newGameServer, "server")
	select {}
}
