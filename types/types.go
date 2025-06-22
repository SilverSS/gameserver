package types

type WSMessage struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
}

type Login struct {
	ClientID int    `json:"clientID"`
	Username string `json:"username"`
}

type Vector struct {
	X float32 `json:"X"`
	Y float32 `json:"Y"`
	Z float32 `json:"Z"`
}

type MoveRequest struct {
	Target Vector `json:"target"`
}

type MoveApproved struct {
	Target Vector  `json:"target"`
	Speed  float32 `json:"speed"`
}

type PositionCorrection struct {
	Position Vector `json:"position"`
}

type PlayerState struct {
	Health    int    `json:"health"`
	Position  Vector `json:"Position"`
	Target    Vector `json:"Target"`
	MoveState int    `json:"moveState"` // 0: Idle, 1: Moving
}
