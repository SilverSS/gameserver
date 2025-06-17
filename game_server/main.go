package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"runtime"
	"strings"
	"sync"

	"github.com/SilverSS/gameserver/types"
	"github.com/anthdm/hollywood/actor"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/semaphore"
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
	select {
	case <-s.done:
		// 이미 cleanup 됨
		return
	default:
		close(s.done)
		s.conn.Close()
		if s.server != nil {
			s.server.removeSession(s.pid)
		}
	}
}

func (s *PlayerSession) readLoop() {
	defer s.cleanup()

	fmt.Printf("client %d : session %d started\n", s.clientID, s.sessionID)

	var msg types.WSMessage
	for {
		select {
		case <-s.done:
			return
		default:
			err := s.conn.ReadJSON(&msg)
			if err != nil {
				// 1. websocket.CloseError 타입인 경우
				if closeErr, ok := err.(*websocket.CloseError); ok {
					switch closeErr.Code {
					case websocket.CloseNormalClosure:
						fmt.Printf("client %d : session %d 정상 종료 (CloseNormalClosure)\n", s.clientID, s.sessionID)
					case websocket.CloseGoingAway:
						fmt.Printf("client %d : session %d 정상 종료 (CloseGoingAway)\n", s.clientID, s.sessionID)
					case websocket.CloseAbnormalClosure:
						fmt.Printf("client %d : session %d 비정상 종료 (CloseAbnormalClosure)\n", s.clientID, s.sessionID)
					default:
						fmt.Printf("client %d : session %d 종료 (code=%d, text=%s)\n", s.clientID, s.sessionID, closeErr.Code, closeErr.Text)
					}
					// 2. 네트워크 연결이 이미 닫힌 경우
				} else if strings.Contains(err.Error(), "use of closed network connection") {
					fmt.Printf("client %d : session %d 네트워크 연결 종료 (use of closed network connection)\n", s.clientID, s.sessionID)
					// 3. 타임아웃 등 기타 네트워크 에러
				} else if strings.Contains(err.Error(), "i/o timeout") {
					fmt.Printf("client %d : session %d 네트워크 타임아웃\n", s.clientID, s.sessionID)
					// 4. 기타 예상치 못한 에러
				} else {
					fmt.Printf("client %d : session %d 예기치 않은 read error: %v\n", s.clientID, s.sessionID, err)
				}
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
		//fmt.Printf("client : %d = %v\n", s.clientID, ps)
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
	mu       sync.Mutex          // 세션 맵 보호용 뮤텍스
	connSem  *semaphore.Weighted // 동시 접속자 제한용 세마포어
}

func newGameServer() actor.Receiver {
	return &GameServer{
		sessions: make(map[*actor.PID]struct{}),
		mu:       sync.Mutex{},
		connSem:  semaphore.NewWeighted(10000), // 최대 10,000명 동시 접속 제한
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
	defer s.mu.Unlock()
	delete(s.sessions, pid)
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
	// 세마포어 획득 시도
	if err := s.connSem.Acquire(r.Context(), 1); err != nil {
		http.Error(w, "서버 동시 접속자 수가 초과되었습니다.", http.StatusServiceUnavailable)
		return
	}
	defer s.connSem.Release(1) // 반드시 반환

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
	// fmt.Println("Processors: ", runtime.NumCPU())

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
