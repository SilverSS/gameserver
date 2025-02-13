package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"runtime"
	"sync"

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
	server    *GameServer
	done      chan struct{}
	pid       *actor.PID
}

// Receive implements actor.Receiver.
func (s *PlayerSession) Receive(c *actor.Context) {
	switch c.Message().(type) {
	case actor.Started:
		s.pid = c.PID()
		s.done = make(chan struct{})
		go s.readLoop()
	case actor.Stopped:
		s.cleanup()
	}
}

func (s *PlayerSession) cleanup() {
	close(s.done)
	s.conn.Close()
	if s.server != nil {
		s.server.removeSession(s.pid)
	}
}

func (s *PlayerSession) readLoop() {
	defer s.cleanup()

	fmt.Printf("client %s : session %d started\n", s.clientID, s.sessionID)

	var msg types.WSMessage
	for {
		select {
		case <-s.done:
			return
		default:
			if err := s.conn.ReadJSON(&msg); err != nil {
				fmt.Printf("read error for session %d: %v\n", s.sessionID, err)
				return
			}
			s.handleMessage(msg)
		}
	}
}

func (s *PlayerSession) handleMessage(msg types.WSMessage) {
	switch msg.Type {
	case "login":
		var loginMsg types.Login
		if err := json.Unmarshal(msg.Data, &loginMsg); err != nil {
			fmt.Printf("login unmarshal error: %v\n", err)
			return
		}
		s.clientID = loginMsg.ClientID
		s.username = loginMsg.Username
	case "playerState":
		var ps types.PlayerState
		if err := json.Unmarshal(msg.Data, &ps); err != nil {
			fmt.Printf("playerState unmarshal error: %v\n", err)
			return
		}
		//fmt.Println(ps)
	}
}

func newPlayerSession(sid int, conn *websocket.Conn, server *GameServer) actor.Producer {
	return func() actor.Receiver {
		return &PlayerSession{
			conn:      conn,
			sessionID: sid,
			server:    server,
		}
	}
}

type GameServer struct {
	ctx      *actor.Context
	sessions map[*actor.PID]struct{}
	mu       sync.RWMutex
}

func newGameServer() actor.Receiver {
	return &GameServer{
		sessions: make(map[*actor.PID]struct{}),
	}
}

func (s *GameServer) Receive(c *actor.Context) {
	switch c.Message().(type) {
	case actor.Started:
		s.startHTTP()
		s.ctx = c
	}
}

func (s *GameServer) removeSession(pid *actor.PID) {
	s.mu.Lock()
	delete(s.sessions, pid)
	s.mu.Unlock()
	fmt.Printf("client with pid %s disconnected\n", pid)
}

func (s *GameServer) startHTTP() {
	fmt.Printf("starting HTTP server on port %s\n", port)
	go func() {
		http.HandleFunc("/ws", s.handleWS)
		strPort := fmt.Sprintf(":%s", port)
		if err := http.ListenAndServe(strPort, nil); err != nil {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 개발 환경을 위한 설정, 프로덕션에서는 적절히 수정 필요
	},
}

func (s *GameServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("ws upgrade err: ", err)
		return
	}

	fmt.Println("new client is trying to connect")
	sid := rand.Intn(math.MaxInt)
	pid := s.ctx.SpawnChild(newPlayerSession(sid, conn, s), fmt.Sprintf("playersession_%d", sid))

	s.mu.Lock()
	s.sessions[pid] = struct{}{}
	s.mu.Unlock()

	fmt.Printf("client with sid %d and pid %s just connected\n", sid, pid)
}

var port string

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	portFlag := flag.String("port", "9160", "<portNumber>")
	flag.Parse()
	port = *portFlag

	e, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		fmt.Printf("failed to create actor engine: %v\n", err)
		return
	}

	e.Spawn(newGameServer, "server")
	select {}
}
