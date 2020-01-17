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

func(p *Player) readJSON(errChan chan bool){
	for {
		select {
		case <-p.stopReadChan:
			return
		default:
			v := message{}
			err := p.conn.ReadJSON(&v)
			if websocket.IsCloseError(err, websocket.CloseAbnormalClosure){
				errChan <- true
				p.readChan <- websocketMessage{msg:nil, err:err}
				return
			}
			if err != nil {
				fmt.Println("There was a WebSocket error:", err)
			}
			p.readChan <- websocketMessage{msg:v, err:nil}
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
