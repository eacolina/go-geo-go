package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"sync"

	"time"
	"os"
	"encoding/json"
	"io/ioutil"
	"math/rand"
)


var ACKNOWLEDGED = "acknowledged"
var QUESTION = "question"
var TIMEOUT = "timeout"
var capitals []Capital

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
	Games map[string] Game
	GamesMux sync.Mutex
	ConnectionsMux sync.Mutex
	Upgrader websocket.Upgrader

}

type Game struct {
	p1 string
	p2 string
	p1_conn *websocket.Conn
	p2_conn *websocket.Conn
	scores map[string] int
	AnswerSemaphore sync.WaitGroup
}

type Capital struct {
	Country string
	City string
}

func (hub *Hub) InitHub(){
	hub.Upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	hub.Upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	hub.Connections = make(map[string] *websocket.Conn)
	hub.Games = make(map[string] Game)
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
	newGame := Game{p1,p2,p1_conn,p2_conn, map[string]int{p1: 0, p2:0}, sync.WaitGroup{}}
	hub.GamesMux.Lock()
	hub.Games[p1+p2] = newGame
	hub.GamesMux.Unlock()
	question := message{ACKNOWLEDGED, "Let the games begin! 😈"}
	p1_conn.WriteJSON(question)
	p2_conn.WriteJSON(question)
	go newGame.play(3)
}

func(g *Game) play(rounds int){
	for i:= 0; i < rounds; i++{
		question := question{"Ecuador", []string{"Quito", "Bogotá", "Lima", "Montevideo"}}
		m := message{QUESTION, question}

		err := g.p1_conn.WriteJSON(m)
		g.AnswerSemaphore.Add(2)
		if err != nil {
			panic(err)
		}
		go g.waitForAnswers(g.p1,g.p1_conn, "Quito", 30*time.Second)

		err = g.p2_conn.WriteJSON(m)
		if err != nil {
			panic(err)
		}
		go g.waitForAnswers(g.p2, g.p2_conn, "Quito", 30*time.Second)
		g.AnswerSemaphore.Wait()
		fmt.Println("both ans received")
		if i < rounds - 1 {
			waitMessage := message{ACKNOWLEDGED, "Next question in 5 seconds"}
			g.p1_conn.WriteJSON(waitMessage)
			g.p2_conn.WriteJSON(waitMessage)
		}
		p1WaitMessage := message{ACKNOWLEDGED, fmt.Sprintf("%s: %d pts / %s: %d pts", g.p1, g.scores[g.p1], g.p2, g.scores[g.p2])}
		p2WaitMessage := message{ACKNOWLEDGED, fmt.Sprintf("%s: %d pts / %s: %d pts", g.p2, g.scores[g.p2], g.p1, g.scores[g.p1])}
		g.p1_conn.WriteJSON(p1WaitMessage)
		g.p2_conn.WriteJSON(p2WaitMessage)
		if i < rounds - 1 {
			time.Sleep(5 * time.Second)
		}
	}
	var endMessage string
	if g.scores[g.p1] > g.scores[g.p2]{
		endMessage = fmt.Sprintf("%s won!", g.p1)
	} else if g.scores[g.p1] < g.scores[g.p2]{
		endMessage = fmt.Sprintf("%s won!", g.p2)
	} else {
		endMessage = "It was a tie!"
	}
	fmt.Println("Game has ended!")
	waitMessage := message{ACKNOWLEDGED, endMessage}
	g.p1_conn.WriteJSON(waitMessage)
	g.p2_conn.WriteJSON(waitMessage)
}

func(g *Game) waitForAnswers(player string, conn *websocket.Conn, rightAnswer string, ttl time.Duration){
	pipe := make(chan message)
	timer := time.NewTimer(ttl)
	start := time.Now()
	go waitForMessage(conn, pipe)
	for {
		select {
		case ans := <-pipe:
			elapsed := time.Now().Sub(start).Seconds()
			m := message{}
			if ans.Content.(string) == rightAnswer{
				score := int((1 - elapsed/ttl.Seconds())* 100)
				g.scores[player] += score
				m.Type = ACKNOWLEDGED
				m.Content = fmt.Sprintf("🌎 You got it right! +%d pts", score)
			} else {
				m.Type = ACKNOWLEDGED
				m.Content = fmt.Sprintf("👎 Someone needs to buy an atlas. +0 pts")
			}
			timer.Stop()
			conn.WriteJSON(m)
			g.AnswerSemaphore.Done()
			return
		case <-timer.C:
			m := message{ TIMEOUT,"It's too late buddy! 😭"}
			conn.WriteJSON(m)
			g.AnswerSemaphore.Done()
			return
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
	_, found1 := hub.Games[p1+p2]
	_, found2 := hub.Games[p2+p1]
	hub.GamesMux.Unlock()
	if found1 || found2 {
		fmt.Println("Found a game")
		return
	}

	for {
		hub.ConnectionsMux.Lock()
		_, ok := hub.Connections[p2]
		hub.ConnectionsMux.Unlock()
		if ok {
			go hub.startGame(p1, p2)
			return
		}
	}
}

func fetchCapitals(filePath string){
	js, err := os.Open(filePath)
	if err != nil{
		panic(err)
	}
	byte,_ := ioutil.ReadAll(js)
	capitals = make([]Capital,0)
	json.Unmarshal(byte,&capitals)
}

func generateQuestion(n int) (question, string){
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	q := question{}
	capitalsSet := make([]Capital,n)
	options := make([]string, n)
	selected := make(map[int]bool)
	for i := 0; i < n; i++{
		index := r1.Intn(len(capitals))
		for selected[index]{
			index = r1.Intn(len(capitals))
			fmt.Println(index)
		}
		selected[index] = true
		capitalsSet[i] = capitals[index]
		options[i] = capitals[index].City
	}

	answer := capitalsSet[ r1.Intn(len(capitalsSet)) ]
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
	fmt.Println(generateQuestion(4))
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
