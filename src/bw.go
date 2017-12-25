package main

import (
	"github.com/gorilla/websocket"
	"net/http"
	"time"
	"log"
	"encoding/json"
	"fmt"
	//"github.com/garyburd/redigo/redis"
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

	//redisClient, err := redis.DialURL("redis://127.0.0.1:6379")
	//if err != nil {
	//	log.Fatal("Redis gone away")
	//}
	//redisClient.Do("SET", "go", "1.9.2")

	fs := http.FileServer(http.Dir("../game"))
	http.Handle("/", fs)

	http.HandleFunc("/ws", handleWebSocket)
	go handleClientMsg()
	log.Println("http server started on :8001")
	err := http.ListenAndServe(":8001", nil)
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
		output, err := logic(&receiver)
		if err != nil {
			reportError(receiver, err)
			continue
		}
		actionSend(receiver, output)
	}
}
func reportError(receiver Receiver, err error) {
	receiver.ws.WriteJSON(err.Error())
	log.Println(err.Error())
}

func actionSend(receiver Receiver, output ServerMsg) {
	switch receiver.ClientMsg.Action {
	case "init":
		initAction(output, receiver.ws)
	case "play":
		controller(receiver, output)
	}
}

func initAction(output ServerMsg, ws *websocket.Conn) {
	ws.WriteJSON(output)
}

func controller(receiver Receiver, output ServerMsg) {
	ws := receiver.ws
	userid, ok := clientIds[ws]
	if !ok {
		log.Println("ws not found")
		return;
	}
	roomid, ok := userRoom[userid]
	if !ok {
		log.Println("room not found")
		return;
	}
	curRoom, ok := rooms[roomid]
	if !ok {
		log.Println("rooms not found")
		return;
	}

	playerAid := curRoom.A
	playerBid := curRoom.B

	playerAws, ok := idClients[playerAid]
	if !ok {
		log.Println("id A not found")
		return;
	}
	playerBws, ok := idClients[playerBid]
	if !ok {
		log.Println("id B not found")
		return;
	}

	playerAws.WriteJSON(output)
	playerBws.WriteJSON(output)
}

func logic(receiver *Receiver) (ServerMsg, error) {
	switch receiver.ClientMsg.Action {
	case "init":
		err := getRole(receiver)
		if err != nil {
			return ServerMsg{}, err
		}
	case "play":
		err := getPlay(receiver)
		if err != nil {
			return ServerMsg{}, err
		}
	}

	tmp := ServerMsg{}
	tmp.Data = receiver.ClientMsg.Data
	tmp.Em = "ok"
	tmp.Ec = 200
	tmp.Time = int(time.Now().Unix())
	return tmp, nil
}

func getRole(receiver *Receiver) error {
	msg := &User{}
	err := json.Unmarshal([]byte(receiver.ClientMsg.Data), msg)
	if err != nil {
		return err
	}
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
	return nil
}

func getPlay(receiver *Receiver) error {
	log.Println(receiver.ClientMsg.Data)
	playData := &PlayData{}
	err := json.Unmarshal([]byte(receiver.ClientMsg.Data), playData)
	if err != nil {
		return err
	}
	log.Println(playData.Role)

	receiver.ClientMsg.Data = fmt.Sprintf("{\"action\":\"play\",\"value\":{\"x\":%d, \"y\":%d, \"role\":%d}}", playData.X, playData.Y, playData.Role)
	return nil
}
