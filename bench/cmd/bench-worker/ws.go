package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

type logMessage struct {
	jobID    int
	text     string
	finished bool
}

var connections = make(map[int][]*websocket.Conn)
var bufMesssages = make(map[int][]string)
var mu sync.Mutex

func startWS(port int) chan logMessage {
	messageCh := make(chan logMessage, 1000)

	go func() {
		for message := range messageCh {
			mu.Lock()
			// if job is finished
			if message.finished {
				// close all connections
				for _, conn := range connections[message.jobID] {
					if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err == nil {
						conn.Close()
					}
				}
				delete(connections, message.jobID)
				delete(bufMesssages, message.jobID)
				mu.Unlock()
				continue
			}
			bufMesssages[message.jobID] = append(bufMesssages[message.jobID], message.text)
			connected := []*websocket.Conn{}
			// check if still connected
			for _, conn := range connections[message.jobID] {
				if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err == nil {
					connected = append(connected, conn)
				}
			}
			connections[message.jobID] = connected
			// send message to living connections
			for _, conn := range connected {
				if err := conn.WriteMessage(websocket.TextMessage, []byte(message.text)); err != nil {
					log.Println(err.Error())
				}
			}
			mu.Unlock()
		}
	}()
	go func() {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		}
		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			jobID, err := strconv.Atoi(r.URL.Query().Get("job"))
			if err != nil {
				log.Println(err.Error())
				return
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println(err.Error())
				return
			}
			mu.Lock()
			defer mu.Unlock()
			// add to connections
			connections[jobID] = append(connections[jobID], conn)

			// send buffered messages
			for _, text := range bufMesssages[jobID] {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(text))
			}
		})
		http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}()
	return messageCh
}
