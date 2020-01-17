package main

import (
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"sync"
	"time"
)

const QUESTION_TIMEOUT  = 30 * time.Second

type Game struct {
	p1              Player
	p2              Player
	scores          map[string]int
	AnswerSemaphore sync.WaitGroup
	CancelGame 		chan bool
}

func (g *Game) play(rounds int) {
	defer g.finishGame()
	for i := 0; i < rounds; i++ {
		select {
		case <-g.CancelGame:
			fmt.Println("Game canceled")
			return
		default:
			q, ans := generateQuestion(4)
			g.playQuestion(q, ans)
			scoreUpdate := message{
				Type:    ScoreUpdate,
				Content: g.scores,
			}
			g.p1.sendJSON(scoreUpdate)
			g.p2.sendJSON(scoreUpdate)
			if i < rounds-1 {
				time.Sleep(1 * time.Second)
			}
		}

	}
}

func (g *Game) playQuestion(question question, answer string){
	m := message{QUESTION, question}
	err := g.p1.sendJSON(m)
	if err != nil {
		g.CancelGame <- true
		return
	}
	g.AnswerSemaphore.Add(2)
	go g.waitForAnswers(g.p1, question, answer, QUESTION_TIMEOUT)
	err2 := g.p2.sendJSON(m)
	if err2 != nil {
		g.CancelGame <- true
		return
	}
	go g.waitForAnswers(g.p2, question, answer, QUESTION_TIMEOUT)
	g.AnswerSemaphore.Wait()
	fmt.Println("both ans received")
}

func (g *Game) finishGame() {
	endMessage := message{}
	endMessage.Type = GAMEOVER
	if g.scores[g.p1.name] > g.scores[g.p2.name] {
		res :=  []string{g.p1.name, g.p2.name}
		endMessage.Content = gameOver{Leaderboard: res}
	} else if g.scores[g.p1.name] < g.scores[g.p2.name] {
		res :=  []string{g.p2.name, g.p1.name}
		endMessage.Content = gameOver{Leaderboard: res}
	} else {
		endMessage.Content = gameOver{Leaderboard: []string{}}
	}
	fmt.Println("Game has ended!")
	fmt.Println(endMessage)
	sendErrP1 := g.p1.sendJSON(endMessage)
	if sendErrP1 != nil {
		fmt.Println("Error sending to", g.p1.name)
		g.CancelGame <- true
	}
	sendErrP2 := g.p2.sendJSON(endMessage)
	if sendErrP2 != nil {
		fmt.Println("Error sending to", g.p2.name)
		g.CancelGame <- true
	}
}

func (g *Game) waitForAnswers(player Player, question question, rightAnswer string, ttl time.Duration) {
	defer g.AnswerSemaphore.Done()
	timer := time.NewTimer(ttl)
	start := time.Now()
	for {
		select {
		case wsMsg := <-player.readChan:
			if wsMsg.err != nil {
				g.CancelGame <- true
				return
			}
			fractionOfTime := time.Now().Sub(start).Seconds()/ttl.Seconds()
			reply, err := g.processAnswer(wsMsg.msg, question, rightAnswer, player, fractionOfTime)
			if err != nil{
				fmt.Println(err)
			}
			timer.Stop()
			sendErr := player.sendJSON(reply)
			if sendErr != nil {
				fmt.Println("Error sending to", player.name)
				g.CancelGame <- true
			}
			return
		case <-timer.C:
			m := message{TIMEOUT, "It's too late buddy! 😭"}
			sendErr := player.sendJSON(m)
			if sendErr != nil {
				fmt.Println("Error sending to", player.name)
				g.CancelGame <- true
			}
			return
		}

	}

}

func (g *Game) processAnswer(msg message, question question, rightAnswer string, player Player, fractionOfTime float64) (message, error){
	if msg.Type != ANSWER {
		fmt.Print("You fucked up")
	}
	answer := answer{}
	mapstructure.Decode(msg.Content, &answer)
	if answer.Id != question.Id {
		return message{}, errors.New("Answer ID doesn't match question ID")
	}
	m := message{}
	m.Type = STATUS
	if answer.Capital == rightAnswer {
		score := int((1 - fractionOfTime) * 100)
		g.scores[player.name] += score
		m.Content = status{
			Result:  true,
			Message: fmt.Sprintf("🌎 You got it right! +%d pts", score),
		}
	} else {
		m.Content = status{
			Result:  false,
			Message: fmt.Sprintf("👎 Someone needs to buy an atlas. +0 pts. Right answer was %s ans you sent %s", rightAnswer, answer.Capital),
		}
	}
	return m, nil
}
