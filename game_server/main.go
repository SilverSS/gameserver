package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/SilverSS/gameserver/ent"
	"github.com/SilverSS/gameserver/ent/user"
	"github.com/SilverSS/gameserver/types"
	"github.com/anthdm/hollywood/actor"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/semaphore"
)

// 서버가 관리하는 이동 속도(유닛/초)
const serverMoveSpeed = 1.0

var jwtSecret = []byte("your_secret_key")

type PlayerSession struct {
	sessionID int
	clientID  int
	username  string
	inLobby   bool
	conn      *websocket.Conn
	server    *GameServer
	done      chan struct{}
	pid       *actor.PID

	state      types.PlayerState // 서버가 관리하는 실제 상태
	target     types.Vector
	moving     bool
	lastUpdate time.Time

	correctionStop chan struct{} // 보정 루프 종료용 채널

	writeMu sync.Mutex // WebSocket Write 보호용 뮤텍스 추가
}

// Receive implements actor.Receiver.
func (s *PlayerSession) Receive(c *actor.Context) {
	switch c.Message().(type) {
	case actor.Started:
		s.pid = c.PID()
		s.done = make(chan struct{})
		s.lastUpdate = time.Now()
		go s.readLoop()
	case actor.Stopped:
		s.cleanup()
	}
}

func (s *PlayerSession) cleanup() {
	s.stopPositionCorrection()
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

// 클라이언트 메시지 처리: 목표 위치, 상태 갱신
func (s *PlayerSession) handleMessage(msg types.WSMessage) {
	switch msg.Type {
	case "moveRequest":
		// 이동 요청 수신: 목표 위치 저장, 이동 상태로 전환, 이동 승인 메시지 전송
		var req types.MoveRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			fmt.Printf("moveRequest unmarshal error: %v\n", err)
			return
		}
		s.target = req.Target
		s.state.Target = req.Target
		s.moving = true
		s.lastUpdate = time.Now()
		// 이동 승인 메시지 전송
		approved := types.MoveApproved{
			Target: req.Target,
			Speed:  serverMoveSpeed,
		}
		sendWS(s.conn, "moveApproved", approved, &s.writeMu)
		// 이동 승인 시점에 세션별 보정 루프 시작
		s.startPositionCorrection()
	}
}

// 세션별 위치 보정 루프 (이동 승인 시점에만 실행)
func (s *PlayerSession) startPositionCorrection() {
	// 이미 실행 중이면 중복 실행 방지
	if s.correctionStop != nil {
		return
	}
	s.correctionStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		defer func() { s.correctionStop = nil }()
		for {
			select {
			case <-ticker.C:
				if !s.moving {
					return // 이동 종료 시 루프 종료
				}
				now := time.Now()
				dt := float32(now.Sub(s.lastUpdate).Seconds())
				s.lastUpdate = now

				cur := s.state.Position
				tgt := s.target
				dir := normalize(subtract(tgt, cur))
				move := multiply(dir, serverMoveSpeed*dt)
				next := add(cur, move)

				// 목표 위치 도달 체크
				if distance(next, tgt) < 0.01 || dot(subtract(tgt, cur), dir) <= 0 {
					next = tgt
					s.moving = false
					s.state.MoveState = 0 // Idle
				} else {
					s.state.MoveState = 1 // Moving
				}
				s.state.Position = next

				// 위치 보정 메시지 전송 (Mutex로 보호)
				correction := types.PositionCorrection{Position: next}
				sendWS(s.conn, "positionCorrection", correction, &s.writeMu)
			case <-s.correctionStop:
				return
			}
		}
	}()
}

// 이동 종료 시 보정 루프 종료 (cleanup 등에서 호출)
func (s *PlayerSession) stopPositionCorrection() {
	if s.correctionStop != nil {
		close(s.correctionStop)
		s.correctionStop = nil
	}
}

// 0.2초마다 이동 상태인 세션의 위치 계산 및 보정 메시지 전송
func (s *GameServer) startPositionCorrection() {
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			<-ticker.C
			s.mu.Lock()
			for pid := range s.sessions {
				session, ok := s.getSessionByPID(pid)
				if ok && session.moving {
					now := time.Now()
					dt := float32(now.Sub(session.lastUpdate).Seconds())
					session.lastUpdate = now

					cur := session.state.Position
					tgt := session.target
					dir := normalize(subtract(tgt, cur))
					move := multiply(dir, serverMoveSpeed*dt)
					next := add(cur, move)

					// 목표 위치 도달 체크
					if distance(next, tgt) < 0.01 || dot(subtract(tgt, cur), dir) <= 0 {
						next = tgt
						session.moving = false
						session.state.MoveState = 0 // Idle
					} else {
						session.state.MoveState = 1 // Moving
					}
					session.state.Position = next

					// 위치 보정 메시지 전송
					correction := types.PositionCorrection{Position: next}
					sendWS(session.conn, "positionCorrection", correction, &session.writeMu)
				}
			}
			s.mu.Unlock()
		}
	}()
}

// 유틸: 메시지 전송 (세션별 Mutex로 보호)
func sendWS(conn *websocket.Conn, msgType string, v interface{}, mu *sync.Mutex) {
	data, _ := json.Marshal(v)
	msg := types.WSMessage{
		Type: msgType,
		Data: data,
	}
	mu.Lock()
	defer mu.Unlock()
	conn.WriteJSON(msg)
}

// 벡터 연산 함수들
func subtract(a, b types.Vector) types.Vector {
	return types.Vector{X: a.X - b.X, Y: a.Y - b.Y, Z: a.Z - b.Z}
}
func add(a, b types.Vector) types.Vector {
	return types.Vector{X: a.X + b.X, Y: a.Y + b.Y, Z: a.Z + b.Z}
}
func multiply(a types.Vector, scalar float32) types.Vector {
	return types.Vector{X: a.X * scalar, Y: a.Y * scalar, Z: a.Z * scalar}
}
func length(a types.Vector) float32 {
	return float32(math.Sqrt(float64(a.X*a.X + a.Y*a.Y + a.Z*a.Z)))
}
func normalize(a types.Vector) types.Vector {
	l := length(a)
	if l == 0 {
		return types.Vector{X: 0, Y: 0, Z: 0}
	}
	return types.Vector{X: a.X / l, Y: a.Y / l, Z: a.Z / l}
}
func distance(a, b types.Vector) float32 {
	return length(subtract(a, b))
}
func dot(a, b types.Vector) float32 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

// getSessionByPID 유틸 함수 예시 (실제 구현 필요)
func (s *GameServer) getSessionByPID(pid *actor.PID) (*PlayerSession, bool) {
	// 실제 구현에 맞게 세션을 찾아 반환해야 함
	return nil, false // 예시
}

// JSON 마샬 유틸
func mustJsonMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
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
	dbClient *ent.Client
}

func newGameServer(dbClient *ent.Client) actor.Receiver {
	return &GameServer{
		sessions: make(map[*actor.PID]struct{}),
		mu:       sync.Mutex{},
		connSem:  semaphore.NewWeighted(10000), // 최대 10,000명 동시 접속 제한
		dbClient: dbClient,
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
		http.HandleFunc("/register", handleRegister)
		http.HandleFunc("/login", handleLogin)
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
	// 1. JWT 토큰 검증
	token := r.URL.Query().Get("token")
	username, err := verifyJWT(token)
	if err != nil {
		http.Error(w, "인증 실패: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// 2. 세마포어 획득 시도
	if err := s.connSem.Acquire(r.Context(), 1); err != nil {
		http.Error(w, "서버 동시 접속자 수가 초과되었습니다.", http.StatusServiceUnavailable)
		return
	}
	defer s.connSem.Release(1)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("ws upgrade err: ", err)
		return
	}

	fmt.Println("new client is trying to connect (user:", username, ")")
	sid := rand.Intn(math.MaxInt)
	pid := s.ctx.SpawnChild(newPlayerSession(sid, conn, s), fmt.Sprintf("playersession_%d", sid))

	s.mu.Lock()
	s.sessions[pid] = struct{}{}
	s.mu.Unlock()

	fmt.Printf("client with sid %d and pid %s just connected (user: %s)\n", sid, pid, username)
}

var port string

// main 함수 내에서 DB 클라이언트를 전역 변수로 할당
var globalDBClient *ent.Client

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	// DB 초기화
	dbClient, err := InitDB()
	if err != nil {
		fmt.Printf("DB 초기화 실패: %v\n", err)
		return
	}
	globalDBClient = dbClient

	portFlag := flag.String("port", "9160", "<portNumber>")
	flag.Parse()
	port = *portFlag

	e, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		fmt.Printf("failed to create actor engine: %v\n", err)
		return
	}

	e.Spawn(func() actor.Receiver { return newGameServer(dbClient) }, "server")
	select {}
}

func (s *PlayerSession) sendRegisterResponse(success bool, msg string) {
	resp := types.RegisterResponse{Success: success, Message: msg}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.conn.WriteJSON(types.WSMessage{Type: "registerResponse", Data: mustJsonMarshal(resp)})
}

func (s *PlayerSession) sendLoginResponse(success bool, msg string) {
	resp := types.LoginResponse{Success: success, Message: msg}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.conn.WriteJSON(types.WSMessage{Type: "loginResponse", Data: mustJsonMarshal(resp)})
}

func hashPassword(pw string) string {
	h := sha256.Sum256([]byte(pw))
	return fmt.Sprintf("%x", h[:])
}

// JWT 토큰 생성
func createJWT(username string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// JWT 토큰 검증
func verifyJWT(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}
	username, ok := claims["username"].(string)
	if !ok {
		return "", fmt.Errorf("username not found in token")
	}
	return username, nil
}

// 회원가입 HTTP 핸들러
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")
	fmt.Printf("[REGISTER] Content-Type: %s\n", r.Header.Get("Content-Type"))
	fmt.Printf("[REGISTER] r.Form: %+v\n", r.Form)
	fmt.Printf("[REGISTER] username: %s, password: %s\n", username, password)
	if username == "" || password == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "필수 입력값 누락"})
		return
	}
	ctx := context.Background()
	client := globalDBClient // 전역 DB 클라이언트
	_, err := client.User.Query().Where(user.UsernameEQ(username)).First(ctx)
	if err == nil {
		fmt.Printf("[REGISTER] 이미 존재하는 사용자명: %s\n", username)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "이미 존재하는 사용자명입니다."})
		return
	}
	_, err = client.User.Create().SetUsername(username).SetPasswordHash(hashPassword(password)).SetCreatedAt(time.Now()).Save(ctx)
	if err != nil {
		fmt.Printf("[REGISTER] DB 오류: %v\n", err)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "DB 오류: " + err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "회원가입 성공!"})
}

// 로그인 HTTP 핸들러
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")
	fmt.Printf("[LOGIN] Content-Type: %s\n", r.Header.Get("Content-Type"))
	fmt.Printf("[LOGIN] r.Form: %+v\n", r.Form)
	fmt.Printf("[LOGIN] username: %s, password: %s\n", username, password)
	if username == "" || password == "" {
		fmt.Printf("[LOGIN] 입력값 누락: username=%s, password=%s\n", username, password)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "필수 입력값 누락"})
		return
	}
	ctx := context.Background()
	client := globalDBClient
	u, err := client.User.Query().Where(user.UsernameEQ(username)).First(ctx)
	if err != nil {
		fmt.Printf("[LOGIN] 존재하지 않는 계정: %s\n", username)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "존재하지 않는 계정입니다."})
		return
	}
	if u.PasswordHash != hashPassword(password) {
		fmt.Printf("[LOGIN] 비밀번호 불일치: username=%s, 입력된 password=%s\n", username, password)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "비밀번호가 일치하지 않습니다."})
		return
	}
	token, err := createJWT(username)
	if err != nil {
		fmt.Printf("[LOGIN] 토큰 생성 실패: %v\n", err)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "토큰 생성 실패"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "token": token})
}
