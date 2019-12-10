package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"sync"

	"time"
)


var ACKNOWLEDGED = "acknowledged"
var QUESTION = "question"
var TIMEOUT = "timeout"

type message struct {
	Type string
	Content interface{}
}

type question struct{
	Country string
	Options []string
}

type Hub struct {
	Connections map[string] *websocket.Conn
	Games map[string] bool
	GamesMux sync.Mutex
	ConnectionsMux sync.Mutex
	Upgrader websocket.Upgrader
}

func (hub *Hub) InitHub(){
	hub.Upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	hub.Upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	hub.Connections = make(map[string] *websocket.Conn)
	hub.Games = make(map[string] bool)
	hub.ConnectionsMux = sync.Mutex{}
	hub.GamesMux = sync.Mutex{}
}

var port = *flag.String("ip", "3434", "help message for flagname")


var connections = make(map [string]*websocket.Conn)



func(hub *Hub) startGame(p1 string, p2 string){
	hub.ConnectionsMux.Lock()
	p1_conn := hub.Connections[p1]
	p2_conn := hub.Connections[p2]
	hub.ConnectionsMux.Unlock()
	question := message{ACKNOWLEDGED, "Let the games begin! 😈"}
	p1_conn.WriteJSON(question)
	p2_conn.WriteJSON(question)
	go hub.play(p1_conn, p2_conn)
}

func(hub *Hub) play(p1_conn, p2_conn *websocket.Conn){
	for i:= 0; i < 1; i++{
		question := question{"Ecuador", []string{"Quito", "Bogotá", "Lima", "Montevideo"}}
		message := message{QUESTION, question}
		err := p1_conn.WriteJSON(message)
		go hub.waitForAnswers(p1_conn, "Quito", 30*time.Second)
		if err != nil {
			panic(err)
		}
		err = p2_conn.WriteJSON(message)
		go hub.waitForAnswers(p2_conn, "Quito", 30*time.Second)
		if err != nil {
			panic(err)
		}

	}
}

func(hub *Hub) waitForAnswers(conn *websocket.Conn, rightAnswer string, ttl time.Duration){
	pipe := make(chan message)
	timer := time.NewTimer(ttl)
	go waitForMessage(conn, pipe)
	for {
		select {
		case ans := <-pipe:
			m := message{}
			if ans.Content.(string) == rightAnswer{
				m.Type = ACKNOWLEDGED
				m.Content = "You got it right! 🌎"
			} else {
				m.Type = ACKNOWLEDGED
				m.Content = "👎 Someone needs to buy an atlas"
			}
			timer.Stop()
			conn.WriteJSON(m)
		case <-timer.C:
			m := message{ TIMEOUT,"It's too late buddy! 😭"}
			conn.WriteJSON(m)
		}

	}

}

func waitForMessage(conn *websocket.Conn, pipe chan message){
	m := message{}
	err := conn.ReadJSON(&m)
	if err != nil {
		panic(err)
	}
	pipe <- m
}

func(hub *Hub) waitForOpponent(p1 string, p2 string){
	hub.GamesMux.Lock()
	if hub.Games[p1+p2] || hub.Games[p2+p1]{
		fmt.Println("Found a game")
		return
	}
	hub.GamesMux.Unlock()
	for {
		hub.ConnectionsMux.Lock()
		res := hub.Connections[p2]
		hub.ConnectionsMux.Unlock()
		if res != nil {
			hub.Games[p1+p2] = true
			go hub.startGame(p1, p2)
			return
		}
	}
}

func main() {
	flag.Parse()
	fmt.Println("Starting server... 🚀")
	hub := Hub{}
	hub.InitHub()

	ws_handler := func(w http.ResponseWriter, r *http.Request) {
		sender := r.Header.Get("sender")
		opponent := r.Header.Get("opponent")

		conn, err := hub.Upgrader.Upgrade(w, r, nil)
		conn.SetCloseHandler(func(code int, text string) error {
			fmt.Println("Connection Closed")
			return nil
		})

		hub.ConnectionsMux.Lock()
		hub.Connections[sender] =  conn
		hub.ConnectionsMux.Unlock()

		go hub.waitForOpponent(sender, opponent)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s started connection\n", sender)
	}
	http.HandleFunc("/ws", ws_handler)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil{
		panic(err)
	}
}
