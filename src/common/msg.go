package common

import "github.com/gorilla/websocket"

type ClientMsg struct {
	Action    string `json:"action"`
	Useragent string `json:"user-agent"`
	Data      string `json:"data"`
}

type Receiver struct {
	ClientMsg ClientMsg
	Ws        *websocket.Conn
}
