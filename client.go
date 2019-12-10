package main

import (
	"fmt"
	"net/http"
	"github.com/gorilla/websocket"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/mapstructure"
)


var to_user string
var from_user string

type Game struct{
	Conn *websocket.Conn
}

type message struct {
	Type string
	Content interface{}
}

type question struct{
	Country string
	Options []string
}


func (game *Game) initGame(socket_url string,  player string,  opponent string){
	game.connectToSocket(socket_url, player, opponent)
}

func (game *Game) connectToSocket(url string, player string, opponent string) {
	header := make(http.Header)
	var Dialer websocket.Dialer

	header.Add("Origin", "http://localhost:3434/")
	header.Add("sender", player)
	header.Add("opponent", opponent)



	conn, resp, err := Dialer.Dial(url, header)

	if err == websocket.ErrBadHandshake {
		fmt.Printf("handshake failed with status %d\n", resp.StatusCode)
		panic(err)
	}
	game.Conn = conn
}



func checkSocket(conn *websocket.Conn){
	for {
		m := message{}
		err := conn.ReadJSON(&m)
		if err != nil {
			fmt.Println(err)
			return
		}
		switch m.Type {
		case "acknowledged":
			fmt.Printf("%s \n",m.Content)
		case "timeout":
			fmt.Printf("%s \n",m.Content)
		case "question":
			q := question{}
			mapstructure.Decode(m.Content, &q)
			ans_p := promptui.Select{
				Label: "",
				Items: q.Options,
				HideSelected:false,
				Templates: &promptui.SelectTemplates{
					Label:fmt.Sprintf("%s {{.}}: ", promptui.IconInitial),
					Selected:fmt.Sprintf(`{{ "What is the capital of %s?" }} {{ . | faint }}`, q.Country),
			},

			}
			_, ans, err := ans_p.Run()
			if err != nil{
				panic(err)
			}
			resp := message{"answer", ans}
			conn.WriteJSON(resp)
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
		Label:"Enter your opponent's username",
	}

	player, err := username_promt.Run()
	opponent, err := opponent_prompt.Run()

	if player == "" || opponent == "" {
		fmt.Println("Please input valid user names")
		return
	}

	game := Game{}
	fmt.Println("Statrting game...🕹")
	game.initGame("ws://localhost:3434/ws", player, opponent)
	checkSocket(game.Conn)


	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}


}