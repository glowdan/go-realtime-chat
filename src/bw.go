package main

import (
	"net/http"
	"log"
	//"github.com/garyburd/redigo/redis"
	"github.com/glowdan/go-realtime-chat/src/logic"
)

func main() {

	//redisClient, err := redis.DialURL("redis://127.0.0.1:6379")
	//if err != nil {
	//	log.Fatal("Redis gone away")
	//}
	//redisClient.Do("SET", "go", "1.9.2")

	fs := http.FileServer(http.Dir("../game"))
	http.Handle("/", fs)

	http.HandleFunc("/ws", logic.HandleWebSocket)
	go logic.HandleClientMsg()
	log.Println("http server started on :8001")
	err := http.ListenAndServe(":8001", nil)
	if err != nil {
		log.Fatal("ListenAndServer:", err)
	}
}
