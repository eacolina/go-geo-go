package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"sync"
)

type Player struct{
	Id           string
	conn         *websocket.Conn
	connMux      sync.Mutex
	readChan     chan websocketMessage
	stopReadChan chan bool
}

func(p *Player) New(playerID string, conn *websocket.Conn){
	p.Id = playerID
	p.conn = conn
	p.connMux = sync.Mutex{}
	p.readChan = make(chan websocketMessage, 4)
	p.stopReadChan = make(chan bool, 2)
}

func(p *Player) readJSON(){
	fmt.Println("starting read for ", p.Id)
	for {
		select {
		case <- p.stopReadChan:
			fmt.Println("stopped read for ", p.Id)
			return
		default:
			v := message{}
			err := p.conn.ReadJSON(&v)
			if err != nil {
				p.readChan <- websocketMessage{msg:message{}, err:err}
				if websocket.IsUnexpectedCloseError(err,websocket.CloseGoingAway,websocket.CloseAbnormalClosure){
					fmt.Println("There was a WebSocket error:", err)
				}
				fmt.Printf("Closed connection for: %s\n", p.Id)
				return
			}
			p.readChan <- websocketMessage{msg:v, err:nil}
		}
	}
}

func(p *Player) sendJSON(v interface{}) error{
	defer p.connMux.Unlock()
	p.connMux.Lock()
	return p.conn.WriteJSON(v)
}

func(p *Player) dropConnection(){
	err := p.conn.Close()
	if err != nil {
		fmt.Println("Error when closing connection:", err)
	}
}
