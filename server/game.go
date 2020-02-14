package main

import (
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"sort"
	"sync"
	"time"
)

const QUESTION_TIMEOUT  = 30 * time.Second

type Game struct {
	Id              int
	Players			[]Player
	NumberOfPlayers int
	NumberOfRounds  int
	OnlinePlayers   int
	scores          map[string]int
	AnswerSemaphore sync.WaitGroup
	JoinSemaphore   sync.WaitGroup
	StopGame        chan bool
	UnregisterGame  chan int
}

func (g *Game) New(numOfPlayers int, numOfRounds int, gameID int, unregisterGame chan int){
	g.Id = gameID
	g.Players = make([] Player, numOfPlayers)
	g.NumberOfPlayers = numOfPlayers
	g.NumberOfRounds = numOfRounds
	g.OnlinePlayers = 0
	g.scores = make(map [string]int)
	g.AnswerSemaphore = sync.WaitGroup{}
	g.JoinSemaphore = sync.WaitGroup{}
	g.StopGame = make(chan bool, 2)
	g.UnregisterGame = unregisterGame
	g.JoinSemaphore.Add(numOfPlayers)
}

func (g *Game) addPlayer(player Player){
	if(g.OnlinePlayers < g.NumberOfPlayers){
		g.Players[g.OnlinePlayers] = player
		g.scores[player.Id] = 0
		g.OnlinePlayers += 1
	} else {
		fmt.Println("You are adding more players than the game assigned")
	}
}

func (g *Game) play() {
	defer g.finishGame()
	for i := 0; i < g.NumberOfRounds; i++ {
		select {
		case <-g.StopGame:
			return
		default:
			q, ans := generateQuestion(4)
			g.playQuestion(q, ans)
			scoreUpdate := message{
				Type:    ScoreUpdate,
				Content: g.scores,
			}
			g.sendMessageToAllPlayers(scoreUpdate)
			if i < g.NumberOfRounds-1 {
				time.Sleep(1 * time.Second)
			}
		}

	}
}

func (g *Game) stopReadingFromAllPlayers(){
	for _, player := range g.Players{
		player.stopReadChan <- true
		fmt.Println("Sent stop for", player.Id)
	}
}

func (g *Game) startReadingFromAllPlayers(){
	for _, player := range g.Players{
		go func(p Player) {
			go p.readJSON()
		}(player)
	}
}

func (g *Game) sendMessageToAllPlayers(msg message){
	for _, player := range g.Players{
		player.sendJSON(msg)
	}
}

func (g *Game) sendQuestionToAllPlayers(q question, answer string){
	m := message{QUESTION, q}
	for _, player := range g.Players{
		err := player.sendJSON(m)
		if err != nil {
			g.StopGame <- true
			return
		}
		g.AnswerSemaphore.Add(1)
		go g.waitForAnswers(player, q, answer, QUESTION_TIMEOUT)
	}
}

func (g *Game) playQuestion(question question, answer string){
	g.sendQuestionToAllPlayers(question, answer)
	g.AnswerSemaphore.Wait()
}

func (g *Game) calculateLeaderbaord(){
	type score struct {
		Player string
		Score int
	}
	var leaderboard []score
	for k, v := range g.scores{
		leaderboard = append(leaderboard, score{k,v})
	}
	sort.Slice(leaderboard, func(i int, j int) bool {return leaderboard[i].Score < leaderboard[j].Score})
	sortedLeaderboard := make(map [string]int)
	for _, s :=  range leaderboard{
		sortedLeaderboard[s.Player] = s.Score
	}
	g.scores = sortedLeaderboard
}

func (g *Game) finishGame() {
	endMessage := message{}
	endMessage.Type = GAMEOVER
	g.calculateLeaderbaord()
	endMessage.Content = g.scores
	g.sendMessageToAllPlayers(endMessage)
	g.stopReadingFromAllPlayers()
	g.UnregisterGame <- g.Id
	fmt.Println("Game has ended!")
}

func (g *Game) waitForAnswers(player Player, question question, rightAnswer string, ttl time.Duration) {
	defer g.AnswerSemaphore.Done()
	timer := time.NewTimer(ttl)
	start := time.Now()
	for {
		select {
		case wsMsg := <-player.readChan:
			if wsMsg.err != nil {
				g.StopGame <- true
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
				fmt.Println("Error sending to", player.Id)
				g.StopGame <- true
			}
			return
		case <-timer.C:
			m := message{TIMEOUT, "It's too late buddy! 😭"}
			sendErr := player.sendJSON(m)
			if sendErr != nil {
				fmt.Println("Error sending to", player.Id)
				g.StopGame <- true
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
		g.scores[player.Id] += score
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
