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

type username string

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
	ws        *websocket.Conn
}

type User struct {
	Username username `json:"username"`
	RoomId   string   `json:"roomid"`
}

type Room struct {
	A username
	B username
}

var clientMsg = make(chan Receiver)
var clients = make(map[*websocket.Conn]bool)
var idClients = make(map[username]*websocket.Conn)
var clientIds = make(map[*websocket.Conn]username)
var userRoom = make(map[username]string)
var isBlack = true
var rooms = make(map[string]*Room)

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
		res := Receiver{ClientMsg: receiver, ws: ws}
		clientMsg <- res
	}
}

func handleClientMsg() {
	for {
		receiver := <-clientMsg
		output := logic(&receiver)

		actionSend(receiver, output)
	}
}

func actionSend(receiver Receiver, output ServerMsg) {
	switch receiver.ClientMsg.Action {
	case "init":
		initAction(output, receiver.ws)
	case "play":
		controller(receiver, output)
		//playAction(output)
	}
}

func initAction(output ServerMsg, ws *websocket.Conn) {
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

func controller(receiver Receiver, output ServerMsg) {
	ws := receiver.ws
	userid := clientIds[ws]
	roomid := userRoom[userid]
	curRoom := rooms[roomid]

	playerAid := curRoom.A
	playerBid := curRoom.B

	playerAws := idClients[playerAid]
	playerBws := idClients[playerBid]

	playerAws.WriteJSON(output)
	playerBws.WriteJSON(output)
}

func logic(receiver *Receiver) ServerMsg {
	switch receiver.ClientMsg.Action {
	case "init":
		getRole(receiver)
	case "play":
		getPlay(receiver)
	}

	tmp := ServerMsg{}
	tmp.Data = receiver.ClientMsg.Data
	tmp.Em = "ok"
	tmp.Ec = 200
	tmp.Time = int(time.Now().Unix())
	return tmp
}

func getRole(receiver *Receiver) {
	msg := &User{}
	json.Unmarshal([]byte(receiver.ClientMsg.Data), msg)
	idClients[msg.Username] = receiver.ws
	clientIds[receiver.ws] = msg.Username

	curRoom, ok := rooms[msg.RoomId]
	if ok {
		curRoom.B = msg.Username
	} else {
		curRoom = &Room{A: msg.Username}
	}
	rooms[msg.RoomId] = curRoom

	userRoom[msg.Username] = msg.RoomId

	if isBlack {
		receiver.ClientMsg.Data = "{\"action\":\"role\",\"value\":1}"
		isBlack = false
	} else {
		receiver.ClientMsg.Data = "{\"action\":\"role\",\"value\":0}"
		isBlack = true
	}
}

func getPlay(receiver *Receiver) {
	log.Println(receiver.ClientMsg.Data)
	playData := &PlayData{}
	json.Unmarshal([]byte(receiver.ClientMsg.Data), playData)
	log.Println(playData.Role)

	receiver.ClientMsg.Data = fmt.Sprintf("{\"action\":\"play\",\"value\":{\"x\":%d, \"y\":%d, \"role\":%d}}", playData.X, playData.Y, playData.Role)
}
