package tests

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
)

var player_username string
var opponent_username string
var charset string = "ABCDEFGHIJKLMNOPQRSTUWVXYZabcdefghijklmnopqrstuvwyz1234567890"
var gameSemaphore sync.WaitGroup = sync.WaitGroup{}

var host *string= flag.String("host", "localhost:3434", "endpoint of game server")

type Game struct {
	Conn *websocket.Conn
}

type message struct {
	Type    string
	Content interface{}
}

type question struct {
	Id string
	Country string
	Options []string
}

type answer struct {
	Id string
	Capital string
}

type status struct {
	Result bool `json:"result"`
	Message string `json:"message"`
}

type gameOver struct{
	Leaderboard []string `json:"leaderboard"`
}

type startGame struct {
	UserID string `json:"userID"`
	GameID string `json:"gameID"`
}

type CreateGameRequest struct {
	Players []string `json:"players"`
	Rounds	int      `json:"rounds"`
}


func playQuestion(q interface{}) answer {
	question := question{}
	mapstructure.Decode(q, &question)
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	index := r1.Intn(len(question.Options))
	return answer{
		Id:      question.Id,
		Capital: question.Options[index],
	}
}

func (game *Game) initGame(socket_url string, player string, opponent string) {

	game.connectToSocket(socket_url, player, opponent)
}

func (game *Game) connectToSocket(url string, player string, opponent string) {
	header := make(http.Header)
	var Dialer websocket.Dialer

	header.Add("Origin", " http://localhost:3434")
	header.Add("userID", player)
	header.Add("gameID", opponent)

	conn, resp, err := Dialer.Dial(url, header)
	if err != nil {
		fmt.Println(err)
	}

	if err == websocket.ErrBadHandshake {
		fmt.Printf("handshake failed with status %d\n", resp.StatusCode)
		panic(err)
	}
	game.Conn = conn
}

func checkSocket(conn *websocket.Conn) {
	for {
		m := message{}
		err := conn.ReadJSON(&m)
		if err != nil {
			fmt.Println(err)
			return
		}
		switch m.Type {
		case "acknowledged":
			fmt.Printf("%s \n", m.Content)
		case "timeout":
			fmt.Printf("%s \n", m.Content)
		case "question":
			ans := playQuestion(m.Content)
			resp := message{"answer", ans}
			conn.WriteJSON(resp)
		case "scoreUpdate":
			scores := make(map[string]int)
			mapstructure.Decode(m.Content, &scores)
		case "status":
			status := status{}
			mapstructure.Decode(m.Content, &status)
			fmt.Println(status.Message)
		case "gameOver":
			g := map[string]int{}
			mapstructure.Decode(m.Content, &g)
			fmt.Println("Game over scores are:")
			if len(g) == 0 {
				fmt.Println("It was a tie")
			} else {
				term := "🏆"
				for k, v := range g{
					fmt.Printf("%s %s: %d\n",term, k, v)
					term = "💩"
				}
			}
		default:
			fmt.Println("Ooops!")
		}

	}
}

func createGameSession(players[] string, rounds int) string{
	createGameEndpoint := fmt.Sprintf("http://%s/game")
	requestBody, err:= json.Marshal(CreateGameRequest{
		Players: players,
		Rounds:  rounds,
	})
	if err != nil {
		fmt.Println(err)
	}
	raw_resp, err :=http.Post(createGameEndpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil{
		fmt.Println(err)
	}
	defer raw_resp.Body.Close()
	respBody, err := ioutil.ReadAll(raw_resp.Body)
	var body map[string]string
	json.Unmarshal(respBody, &body)
	gameID := body["gameID"]
	if (*raw_resp).StatusCode != http.StatusCreated{
		panic("Couldn't create game")
	}
	return gameID
}

func generateRandomInt(upperBound int) int{
	generator := rand.New(rand.NewSource(time.Now().UnixNano()))
	return generator.Intn(upperBound)
}

func generateStringWithCharset(charset string, length int) string{
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[generateRandomInt(len(charset))]
	}
	return string(b)
}

func playGame(ws_endpoint string, player string, gameID string){
	//game := Game{}
	fmt.Printf("%s: started game...🕹\n", player)
	//game.initGame(ws_endpoint, player, gameID)
	//checkSocket(game.Conn)
}

func createMasterGame(ws_endpoint string){
	players := make([]string, generateRandomInt(4))
	for i:=0; i < len(players); i++{
		players[i] = generateStringWithCharset(charset, 5)
	}
	gameID := createGameSession(players, generateRandomInt(11))
	for i:=0; i < len(players); i++{
		playGame(ws_endpoint, players[i], gameID)
	}
	fmt.Printf("Started game %s...🕹\n",gameID)
}

func main() {
	flag.Parse()
	ws_endpoint := fmt.Sprintf("ws://%s/ws",*host)
	fmt.Println(ws_endpoint)
	//gameSemaphore.Add(1)
	createMasterGame(ws_endpoint)
	//gameSemaphore.Wait()
}
