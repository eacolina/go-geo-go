package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
)

type Hub struct {
	Connections    map[string]*websocket.Conn
	Games          map[string]Game
	GamesMux       sync.Mutex
	ConnectionsMux sync.Mutex
	Upgrader       websocket.Upgrader
	Handler http.HandlerFunc
}

func (hub *Hub) InitHub() {
	hub.Handler = func(w http.ResponseWriter, r *http.Request) {
		sender := r.Header.Get("sender")
		opponent := r.Header.Get("opponent")

		conn, err := hub.Upgrader.Upgrade(w, r, nil)
		conn.SetCloseHandler(func(code int, text string) error {
			fmt.Println("Connection Closed")
			return nil
		})

		hub.ConnectionsMux.Lock()
		hub.Connections[sender] = conn
		hub.ConnectionsMux.Unlock()

		go hub.waitForOpponent(sender, opponent)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s started connection\n", sender)
	}
	hub.Upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	hub.Upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	hub.Connections = make(map[string]*websocket.Conn)
	hub.Games = make(map[string]Game)
	hub.ConnectionsMux = sync.Mutex{}
	hub.GamesMux = sync.Mutex{}
}

func (hub *Hub) startGame(game *Game) {
	initMessage := message{ACKNOWLEDGED, "Let the games begin! 😈"}
	game.p1.sendJSON(initMessage)
	game.p2.sendJSON(initMessage)

	go game.p1.readJSON()
	go game.p2.readJSON()

	go game.play(10)
}

func (hub *Hub) waitForOpponent(p1 string, p2 string) {
	for {
		hub.GamesMux.Lock()
		_, found1 := hub.Games[p1+p2]
		_,found2 := hub.Games[p2+p1]
		if found1 || found2 {
			fmt.Println("Found a game")
			return
		}
		hub.ConnectionsMux.Lock()
		_, ok := hub.Connections[p2]
		hub.ConnectionsMux.Unlock()
		if ok {
			player1 := Player{p1, hub.Connections[p1], sync.Mutex{}, make(chan message, 2), make(chan bool)}
			player2 := Player{p2, hub.Connections[p2], sync.Mutex{}, make(chan message, 2), make(chan bool)}
			newGame := Game{player1, player2,map[string]int{p1: 0, p2: 0}, sync.WaitGroup{}}
			hub.Games[p1+p2] = newGame
			go hub.startGame(&newGame)
		}
		hub.GamesMux.Unlock()
		if ok {
			return
		}
	}
}
