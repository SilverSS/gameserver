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
	X float32
	Y float32
	Z float32
}

type PlayerState struct {
	Health   int    `json:health`
	Position Vector `json:Position`
}
