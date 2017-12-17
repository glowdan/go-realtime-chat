package main

import (
	"github.com/gorilla/websocket"
	"net/http"
	"time"
	"log"
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
)

type ServerMsg struct {
	Em   string `json:"em"`
	Ec   int    `json:"ec"`
	Time int    `json:"time"`
	Data string `json:"data"`
}

type ClientMsg struct {
	Action    string `json:"action"`
	Useragent string `json:"user-agent"`
	Data      string `json:"data"`
}

type PlayData struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Role int `json:"role"`
}

type Receiver struct {
	ClientMsg ClientMsg
	ws        websocket.Conn
}

var clientMsg = make(chan Receiver)
var clients = make(map[*websocket.Conn]bool)
var isBlack = true
var currentPlayer = 1

func main() {

	redisClient, err := redis.DialURL("redis://127.0.0.1:6379")
	if err != nil {
		log.Fatal("Redis gone away");
	}
	redisClient.Do("SET", "go", "1.9.2")

	fs := http.FileServer(http.Dir("../game"))
	http.Handle("/", fs)

	http.HandleFunc("/ws", handleWebSocket)
	go handleClientMsg()
	log.Println("http server started on :8001")
	err = http.ListenAndServe(":8001", nil)
	if err != nil {
		log.Fatal("ListenAndServer:", err)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return true
	}}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}

	defer ws.Close()

	clients[ws] = true
	for {
		var receiver ClientMsg
		err := ws.ReadJSON(&receiver)
		if err != nil {
			log.Printf("error:%v", err)
			delete(clients, ws)
			break
		}
		res := Receiver{ClientMsg: receiver, ws: *ws}
		clientMsg <- res
	}
}

func handleClientMsg() {
	for {
		middle := <-clientMsg
		output := logic(middle.ClientMsg)

		actionSend(middle.ClientMsg, output, middle.ws)
	}
}

func actionSend(tmp ClientMsg, output ServerMsg, ws websocket.Conn) {
	switch tmp.Action {
	case "init":
		initAction(output, ws)
	case "play":
		playAction(output)
	}

}

func initAction(output ServerMsg, ws websocket.Conn) {
	ws.WriteJSON(output)
}
func playAction(output ServerMsg) {
	for client := range clients {
		err := client.WriteJSON(output)
		if err != nil {
			log.Printf("error %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func logic(data ClientMsg) ServerMsg {
	switch data.Action {
	case "init":
		getRole(&data)
	case "play":
		getPlay(&data)
	}

	tmp := ServerMsg{}
	tmp.Data = data.Data
	tmp.Em = "ok"
	tmp.Ec = 200
	tmp.Time = int(time.Now().Unix())
	return tmp
}

func getRole(data *ClientMsg) {
	if isBlack {
		data.Data = "{\"action\":\"role\",\"value\":1}"
		isBlack = false
	} else {
		data.Data = "{\"action\":\"role\",\"value\":0}"
		isBlack = true
	}
}

func getPlay(data *ClientMsg) {
	log.Println(data.Data)
	playData := &PlayData{}
	json.Unmarshal([]byte(data.Data), playData)
	log.Println(playData.Role)

	data.Data = fmt.Sprintf("{\"action\":\"play\",\"value\":{\"x\":%d, \"y\":%d, \"role\":%d}}", playData.X, playData.Y, playData.Role)
}
