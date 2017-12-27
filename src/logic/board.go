package logic

import (
	"time"
	"log"
	"fmt"
	"github.com/gorilla/websocket"
	"go-realtime-chat/src/common"
	"net/http"
	"encoding/json"
	"strings"
)

type ServerMsg struct {
	Em   string `json:"em"`
	Ec   int    `json:"ec"`
	Time int    `json:"time"`
	Data string `json:"data"`
}

type PlayData struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Role int `json:"role"`
}

type Room struct {
	A username
	B username
}
type username string

type User struct {
	Username username `json:"username"`
	RoomId   string   `json:"roomid"`
}
type Trace struct {
	Player username
	Time   time.Time
	X, Y   int
}

var idClients = make(map[username]*websocket.Conn)
var clientIds = make(map[*websocket.Conn]username)
var userRoom = make(map[username]string)
var isBlack = true
var rooms = make(map[string]*Room)
var clientMsg = make(chan common.Receiver)
var clients = make(map[*websocket.Conn]bool)
var trace = make([]*Trace, 0)

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
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
		var receiver common.ClientMsg
		err := ws.ReadJSON(&receiver)
		if err != nil {
			log.Printf("error:%v", err)
			delete(clients, ws)
			break
		}
		res := common.Receiver{ClientMsg: receiver, Ws: ws}
		clientMsg <- res
	}
}

func HandleClientMsg() {
	for {
		receiver := <-clientMsg
		output := logic(&receiver)

		actionSend(receiver, output)
	}
}

func actionSend(receiver common.Receiver, output ServerMsg) {
	switch receiver.ClientMsg.Action {
	case "init":
		initAction(output, receiver.Ws)
	case "play":
		controller(receiver, output)
	}
}

func initAction(output ServerMsg, ws *websocket.Conn) {
	ws.WriteJSON(output)
}

func controller(receiver common.Receiver, output ServerMsg) {
	ws := receiver.Ws
	userid, ok := clientIds[ws]
	if !ok {
		log.Println("userid not found")
		return
	}
	roomid, ok := userRoom[userid]
	if !ok {
		log.Println("roomid not found")
		return
	}
	curRoom, ok := rooms[roomid]
	if !ok {
		log.Println("room not found")
		return
	}

	playerAid := curRoom.A
	playerBid := curRoom.B

	playerAws, ok := idClients[playerAid]
	if !ok {
		log.Println("A not found")
		return
	}
	playerBws, ok := idClients[playerBid]
	if !ok {
		log.Println("B not found")
		return
	}

	playerAws.WriteJSON(output)
	playerBws.WriteJSON(output)
}

func logic(receiver *common.Receiver) ServerMsg {
	switch receiver.ClientMsg.Action {
	case "init":
		getRole(receiver)
	case "pre-data":
	case "play":
		getPre(receiver)
		getPlay(receiver)
	}

	tmp := ServerMsg{}
	tmp.Data = receiver.ClientMsg.Data
	tmp.Em = "ok"
	tmp.Ec = 200
	tmp.Time = int(time.Now().Unix())
	return tmp
}

func getPre(receiver *common.Receiver) {

	strArr := make([]string, 0)
	for _, val := range trace {
		tmp := fmt.Sprintf("{\"x\":%d, \"y\":%d, \"user\":\"%s\"}", val.X, val.Y, val.Player)
		strArr = append(strArr, tmp)
	}
	str := strings.Join(strArr, ",")
	fmt.Println("[" + str + "]")
}

func getRole(receiver *common.Receiver) {
	msg := &User{}
	json.Unmarshal([]byte(receiver.ClientMsg.Data), msg)
	idClients[msg.Username] = receiver.Ws
	clientIds[receiver.Ws] = msg.Username

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

func getPlay(receiver *common.Receiver) {
	log.Println(receiver.ClientMsg.Data)
	playData := &PlayData{}
	json.Unmarshal([]byte(receiver.ClientMsg.Data), playData)
	log.Println(playData.Role)

	ws := receiver.Ws
	userid, ok := clientIds[ws]
	if !ok {
		log.Println("userid not found")
		return
	}
	newTrace := Trace{Player: userid, Time: time.Now(), X: playData.X, Y: playData.Y}
	trace = append(trace, &newTrace)

	receiver.ClientMsg.Data = fmt.Sprintf("{\"action\":\"play\",\"value\":{\"x\":%d, \"y\":%d, \"role\":%d}}", playData.X, playData.Y, playData.Role)
}
