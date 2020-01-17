package main

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"sync"
	"time"
)

const QUESTION_TIMEOUT  = 10 * time.Second

type Game struct {
	p1              Player
	p2              Player
	scores          map[string]int
	AnswerSemaphore sync.WaitGroup
}

func (g *Game) play(rounds int) {
	for i := 0; i < rounds; i++ {
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
	g.finishGame()
}

func (g *Game) playQuestion(question question, answer string){
	m := message{QUESTION, question}
	g.p1.sendJSON(m)
	g.AnswerSemaphore.Add(2)
	go g.waitForAnswers(g.p1, question, answer, QUESTION_TIMEOUT)
	g.p2.sendJSON(m)
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
	g.p1.sendJSON(endMessage)
	g.p2.sendJSON(endMessage)
}

func (g *Game) waitForAnswers(player Player, question question, rightAnswer string, ttl time.Duration) {
	defer func (){
		g.AnswerSemaphore.Done()
	}()
	timer := time.NewTimer(ttl)
	start := time.Now()
	for {
		select {
		case wsMsg := <-player.readChan:
			g.handleMessage(wsMsg, question, start, rightAnswer, ttl, player, timer)
			return
		case <-timer.C:
			m := message{TIMEOUT, "It's too late buddy! 😭"}
			player.sendJSON(m)
			return
		}

	}

}

func (g *Game) handleMessage(wsMsg websocketMessage, question question, start time.Time, rightAnswer string, ttl time.Duration, player Player, timer *time.Timer) {
	msg := wsMsg.msg
	if msg.Type != ANSWER {
		fmt.Print("You fucked up")
	}
	answer := answer{}
	mapstructure.Decode(msg.Content, &answer)
	if answer.Id != question.Id {
		fmt.Println("Just skipped an answer")
		return
	}
	elapsed := time.Now().Sub(start).Seconds()
	m := message{}
	m.Type = STATUS
	if answer.Capital == rightAnswer {
		score := int((1 - elapsed/ttl.Seconds()) * 100)
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
	timer.Stop()
	player.sendJSON(m)
	return
}
