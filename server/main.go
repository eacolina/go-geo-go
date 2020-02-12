package main

import (
	"flag"
	"fmt"
	"net/http"
	"path/filepath"

	"encoding/json"
	"github.com/google/uuid"
	"io/ioutil"
	"math/rand"
	"time"
)

var ACKNOWLEDGED = "acknowledged"
var QUESTION = "question"
var TIMEOUT = "timeout"
var ANSWER = "answer"
var STATUS = "status"
var ScoreUpdate = "scoreUpdate"
var GAMEOVER = "gameOver"

var CapitalsFile = "server/assets/countries.json"
var capitals []country

var port = *flag.String("ip", "3434", "help message for flagname")

type websocketMessage struct {
	msg message
	err error
}

type message struct {
	Type    string `json:"type"`
	Content interface{} `json:"content"`
}

type answer struct {
	Id string
	Capital string
}

type question struct {
	Id 		string `json:"id"`
	Country string `json:"country"`
	Options []string `json:"options"`
}

type status struct {
	Result bool `json:"result"`
	Message string `json:"message"`
}

type country struct {
	Name    string
	Capital string
}

type gameOver struct{
	Leaderboard map[string]int `json:"leaderboard"`
}


func fetchCapitals(p string) {
	filePath,_ := filepath.Abs(p)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
	capitals = make([]country, 0)
	json.Unmarshal(data, &capitals)
}

func generateQuestion(n int) (question, string) {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	q := question{}
	capitalsSet := make([]country, n)
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
		options[i] = capitals[index].Capital
	}

	answer := capitalsSet[r1.Intn(len(capitalsSet))]
	q.Id = uuid.New().String()
	q.Country = answer.Name
	q.Options = options
	return q, answer.Capital

}

func main() {
	flag.Parse()
	fmt.Println("Starting server... 🚀")
	var hub = Hub{}
	hub.InitHub()
	fetchCapitals(CapitalsFile)
	http.HandleFunc("/ws", hub.Handler)
	http.HandleFunc("/game", hub.CreateGame)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
