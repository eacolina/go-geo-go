package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"sync"
)

type Player struct{
	name string
	conn *websocket.Conn
	connMux sync.Mutex
	readChan chan websocketMessage
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
				p.readChan <- websocketMessage{msg:message{}, err:err}
				fmt.Println("There was a WebSocket error:", err)
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
