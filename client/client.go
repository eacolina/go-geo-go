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
	header.Add("sender", player)
	header.Add("opponent", opponent)

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
			fmt.Printf("%s: %d pts, %s: %d pts\n", player_username, scores[player_username], opponent_username, scores[opponent_username])
		case "status":
			status := status{}
			mapstructure.Decode(m.Content, &status)
			fmt.Print(status.Message)
		case "gameOver":
			g := gameOver{}
			mapstructure.Decode(m.Content, &g)
			fmt.Print("Game over scores are:")
			if len(g.Leaderboard) == 0 {
				fmt.Println("It was a tie")
			} else {
				fmt.Println(g.Leaderboard)
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
	opponent_prompt := promptui.Prompt{
		Label: "Enter your opponent's username",
	}

	player, err := username_promt.Run()
	opponent, err := opponent_prompt.Run()

	player_username = player
	opponent_username = opponent

	if player == "" || opponent == "" {
		fmt.Println("Please input valid user names")
		return
	}

	game := Game{}
	fmt.Println("Starting game...🕹")
	game.initGame("ws://localhost:3434/ws", player, opponent)
	checkSocket(game.Conn)

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

}
