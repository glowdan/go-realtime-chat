package main

import (
	"github.com/gorilla/websocket"
	"net/http"
	"time"
	"log"
	"encoding/json"
	"fmt"
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

var clientMsg = make(chan ClientMsg)
var clients = make(map[*websocket.Conn]bool)
var isBlack = true

func main() {
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
		clientMsg <- receiver
	}
}

func handleClientMsg() {
	for {
		middle := <-clientMsg
		output := logic(middle)
		for client := range clients {
			err := client.WriteJSON(output)
			if err != nil {
				log.Printf("error %v", err)
				client.Close()
				delete(clients, client)
			}
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
