package main

import (
	"flag"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"net/http"

	"sync"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"

	"encoding/json"
	"io/ioutil"
	"math/rand"
	"os"
	"time"
)

var ACKNOWLEDGED = "acknowledged"
var QUESTION = "question"
var TIMEOUT = "timeout"
var ANSWER = "answer"
var capitals []Capital

type message struct {
	Type    string
	Content interface{}
}

type answer struct {
	Id string
	Capital string
}

type question struct {
	Id 		string
	Country string
	Options []string
}

type Hub struct {
	Connections    map[string]*websocket.Conn
	Games          map[string]Game
	GamesMux       sync.Mutex
	ConnectionsMux sync.Mutex
	Upgrader       websocket.Upgrader
}

type Player struct{
	name string
	conn *websocket.Conn
	connMux sync.Mutex
	readChan chan message
	stopReadChan chan bool
}

func(p *Player) readJSON(){
	for {
		select {
		case <-p.stopReadChan:
			return
		default:
			v := message{}
			err := p.conn.ReadJSON(&v)
			if err != nil {
				panic(err)
			}
			p.readChan <- v
		}
	}
}

func(p *Player) sendJSON(v interface{}){
	defer p.connMux.Unlock()
	p.connMux.Lock()
	err := p.conn.WriteJSON(v)
	if err != nil{
		panic(err)
	}
}

type Game struct {
	p1              Player
	p2              Player
	scores          map[string]int
	AnswerSemaphore sync.WaitGroup
}

type Capital struct {
	Country string
	City    string
}

func (hub *Hub) InitHub() {
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

var port = *flag.String("ip", "3434", "help message for flagname")

var connections = make(map[string]*websocket.Conn)

func (hub *Hub) startGame(game *Game) {
	initMessage := message{ACKNOWLEDGED, "Let the games begin! 😈"}
	game.p1.sendJSON(initMessage)
	game.p2.sendJSON(initMessage)

	go game.p1.readJSON()
	go game.p2.readJSON()

	go game.play(10)
}

func (g *Game) play(rounds int) {
	for i := 0; i < rounds; i++ {
		q, ans := generateQuestion(4)
		m := message{QUESTION, q}

		g.p1.sendJSON(m)
		g.AnswerSemaphore.Add(2)
		go g.waitForAnswers(g.p1, q, ans, 10*time.Second)
		g.p2.sendJSON(m)
		go g.waitForAnswers(g.p2, q, ans, 10*time.Second)
		g.AnswerSemaphore.Wait()
		fmt.Println("both ans received")
		if i < rounds-1 {
			waitMessage := message{ACKNOWLEDGED, "Next question in 5 seconds"}
			g.p1.sendJSON(waitMessage)
			g.p2.sendJSON(waitMessage)
		}
		p1WaitMessage := message{ACKNOWLEDGED, fmt.Sprintf("%s: %d pts / %s: %d pts", g.p1.name, g.scores[g.p1.name], g.p2.name, g.scores[g.p2.name])}
		p2WaitMessage := message{ACKNOWLEDGED, fmt.Sprintf("%s: %d pts / %s: %d pts", g.p2.name, g.scores[g.p2.name], g.p1.name, g.scores[g.p1.name])}
		g.p1.sendJSON(p1WaitMessage)
		g.p2.sendJSON(p2WaitMessage)
		if i < rounds-1 {
			time.Sleep(5 * time.Second)
		}
	}
	var endMessage string
	if g.scores[g.p1.name] > g.scores[g.p2.name] {
		endMessage = fmt.Sprintf("%s won!", g.p1)
	} else if g.scores[g.p1.name] < g.scores[g.p2.name] {
		endMessage = fmt.Sprintf("%s won!", g.p2)
	} else {
		endMessage = "It was a tie!"
	}
	fmt.Println("Game has ended!")
	waitMessage := message{ACKNOWLEDGED, endMessage}
	g.p1.sendJSON(waitMessage)
	g.p2.sendJSON(waitMessage)
}

func (g *Game) waitForAnswers(player Player, question question, rightAnswer string, ttl time.Duration) {
	timer := time.NewTimer(ttl)
	start := time.Now()
	for {
		select {
		case msg := <-player.readChan:
			if msg.Type != ANSWER{
				fmt.Print("You fucked up")
			}
			answer := answer{}
			mapstructure.Decode(msg.Content,&answer)
			if answer.Id != question.Id{
				continue
			}
			elapsed := time.Now().Sub(start).Seconds()
			m := message{}
			m.Type = ACKNOWLEDGED
			if answer.Capital == rightAnswer {
				score := int((1 - elapsed/ttl.Seconds()) * 100)
				g.scores[player.name] += score
				m.Content = fmt.Sprintf("🌎 You got it right! +%d pts", score)
			} else {
				m.Content = fmt.Sprintf("👎 Someone needs to buy an atlas. +0 pts. Right answer was %s ans you sent %s", rightAnswer, answer.Capital)
			}
			timer.Stop()
			player.sendJSON(m)
			g.AnswerSemaphore.Done()
			return
		case <-timer.C:
			m := message{TIMEOUT, "It's too late buddy! 😭"}
			player.sendJSON(m)
			g.AnswerSemaphore.Done()
			return
		}

	}

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

func fetchCapitals(filePath string) {
	js, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	byte, _ := ioutil.ReadAll(js)
	capitals = make([]Capital, 0)
	json.Unmarshal(byte, &capitals)
}

func generateQuestion(n int) (question, string) {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	q := question{}
	capitalsSet := make([]Capital, n)
	options := make([]string, n)
	selected := make(map[int]bool)
	for i := 0; i < n; i++ {
		index := r1.Intn(len(capitals))
		for selected[index] {
			index = r1.Intn(len(capitals))
			fmt.Println(index)
		}
		selected[index] = true
		capitalsSet[i] = capitals[index]
		options[i] = capitals[index].City
	}

	answer := capitalsSet[r1.Intn(len(capitalsSet))]
	q.Id = uuid.New().String()
	q.Country = answer.Country
	q.Options = options
	return q, answer.City

}

func main() {
	flag.Parse()
	fmt.Println("Starting server... 🚀")
	hub := Hub{}
	hub.InitHub()
	fetchCapitals("countries.json")
	ws_handler := func(w http.ResponseWriter, r *http.Request) {
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
	http.HandleFunc("/ws", ws_handler)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
