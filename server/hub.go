package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Hub struct {
	Connections    map[string]*websocket.Conn
	Games          sync.Map
	PlayerGameMap  sync.Map
	GamesMux       sync.Mutex
	ConnectionsMux sync.Mutex
	Upgrader       websocket.Upgrader
	Handler 	   http.HandlerFunc
	CreateGame 	   http.HandlerFunc
	UnregisterGame chan int
}

type CreateGameRequest struct {
	Players []string `json:"players"`
	Rounds	int      `json:rounds`
}

type CreateGameResponse struct {
	GameID int `json:"gameID"`
}

func (hub *Hub) InitHub() {
	hub.Handler = func(w http.ResponseWriter, r *http.Request) {
		playerID := r.Header.Get("userID")
		gameID, _ := strconv.Atoi(r.Header.Get("gameID"))
		retrievedGameID, ok := hub.PlayerGameMap.Load(playerID)

		if ok && retrievedGameID == gameID {
			wsConnection, err := hub.Upgrader.Upgrade(w, r, nil)
			if err != nil {
				panic(err)
			}
			wsConnection.SetCloseHandler(func(code int, text string) error {
				fmt.Println("Connection Closed")
				return nil
			})
			hub.ConnectionsMux.Lock()
			hub.Connections[playerID] = wsConnection
			player := Player{}
			player.New(playerID, hub.Connections[playerID])
			hub.ConnectionsMux.Unlock()
			foundGame, _ := hub.Games.Load(gameID)
			game := foundGame.(*Game)
			game.addPlayer(player)
			game.JoinSemaphore.Done()
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
	hub.CreateGame = func(w http.ResponseWriter, r *http.Request){
		var gameRequest CreateGameRequest
		var game *Game
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		json.Unmarshal(body, &gameRequest)

		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)
		gameID := r1.Intn(10000)

		game = new(Game)
		game.New(len(gameRequest.Players), gameRequest.Rounds, gameID, hub.UnregisterGame)
		hub.Games.Store(gameID, game)

		for _, playerID := range gameRequest.Players{
			hub.PlayerGameMap.Store(playerID, gameID)
		}
		fmt.Printf("Created game %d with players:%v\n", gameID, gameRequest.Players)
		respBody := CreateGameResponse{GameID: gameID}
		respData, err := json.Marshal(respBody)
		if err != nil {
			panic(err)
		}
		go hub.startGame(game)
		w.WriteHeader(http.StatusCreated)
		w.Write(respData)
	}

	hub.Upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	hub.Upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	hub.Connections = make(map[string]*websocket.Conn)
	hub.Games = sync.Map{}
	hub.ConnectionsMux = sync.Mutex{}
	hub.GamesMux = sync.Mutex{}
	hub.UnregisterGame = make(chan int, 20)
	go hub.start()
}

func (hub *Hub) start(){
	for {
		select{
			case id := <-hub.UnregisterGame:
				//fmt.Println("asked to cancel game: ", id)
				g, _ := hub.Games.Load(id)
				hub.finishGame(g.(*Game))
		}
	}
}

func (hub *Hub) startGame(game *Game) {
	initMessage := message{ACKNOWLEDGED, "Let the games begin! 😈"}
	game.JoinSemaphore.Wait()
	fmt.Println("Starting Game")
	game.startReadingFromAllPlayers()
	game.sendMessageToAllPlayers(initMessage)
	go game.play()
}

func(hub *Hub) finishGame(game *Game){
	for _, player := range game.Players{
		player.dropConnection()
		hub.PlayerGameMap.Delete(player.Id)
		hub.ConnectionsMux.Lock()
		delete(hub.Connections, player.Id)
		hub.ConnectionsMux.Unlock()
	}
	hub.Games.Delete(game.Id)
	fmt.Printf("Cleared game %d succesfully\n", game.Id)
}

