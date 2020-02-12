package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/mapstructure"
)

var player_username string
var opponent_username string

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


func playQuestion(q interface{}) answer {
	question := question{}
	mapstructure.Decode(q, &question)
	question_prompt := fmt.Sprintf("What is the capital of %s?", question.Country)
	ans_p := promptui.Select{
		Label:        question_prompt,
		Items:        question.Options,
		HideSelected: false,
		Templates: &promptui.SelectTemplates{
			Selected: fmt.Sprintf(`{{ "%s" }} {{ . | faint }}`, question_prompt),
		},
	}
	_, ans, err := ans_p.Run()
	if err != nil {
		panic(err)
	}
	return answer{
		Id:      question.Id,
		Capital: ans,
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
			//fmt.Print("Scores :")
			//fmt.Println(scores)
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


func main() {

	username_promt := promptui.Prompt{
		Label: "Input your username",
	}
	gameID_prompt := promptui.Prompt{
		Label: "Enter ID of game room",
	}

	player, err := username_promt.Run()
	gameID, err := gameID_prompt.Run()


	player_username = player
	opponent_username = "John"

	if player == "" || gameID == "" {
		fmt.Println("Please input valid user names")
		return
	}

	game := Game{}
	fmt.Println("Starting game...🕹")
	game.initGame("ws://ee60a3ab.ngrok.io/ws", player, gameID)
	checkSocket(game.Conn)

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

}
