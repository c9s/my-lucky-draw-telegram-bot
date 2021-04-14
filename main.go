package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/joho/godotenv"

	tb "gopkg.in/tucnak/telebot.v2"
	"gopkg.in/yaml.v3"
)

func mentionUser(user *tb.User) string {
	if len(user.Username) > 0 {
		return "@" + user.Username
	}

	name := user.FirstName + " " + user.LastName
	return fmt.Sprintf("[%s](tg://user?id=%d)", name, user.ID)
}

var englishNumbers = []string{
	"",
	"first",
	"second",
	"third",
	"fourth",
	"fifth",
	"sixth",
	"seventh",
	"eighth",
	"ninth",
	"tenth",
}

var tossCupImgs = []string{
	"https://i.imgur.com/Lrfi37a.jpg",
	"https://i.imgur.com/HAb9sjh.jpg",
	"https://i.imgur.com/Fy5hmD7.jpg",
	"https://i.imgur.com/fPuAt7T.jpg",
}

type H map[string]interface{}

func translateOrdinalNumberToEnglish(number int) string {
	if number+1 < len(englishNumbers) {
		return englishNumbers[number]
	}

	return fmt.Sprintf("%dth", number)
}

type PrizeEntry struct {
	Name     string
	Quantity int
	Winners  []*tb.User
}

func readConfig(configFile string) (*Config, error) {
	var conf Config
	yamlFile, err := ioutil.ReadFile(configFile)

	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(yamlFile, &conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}

type DrawSession struct {
	mu sync.Mutex

	Message                 *tb.Message
	Organizer               *tb.User
	JoinedMembers           map[int]*tb.User
	WinningMembers          map[int]*tb.User
	MemberIDList            []int
	PrizeEntries            []PrizeEntry
	JoinDuration            time.Duration
	PrizeAnnouncementDelay  time.Duration
	WinnerAnnouncementDelay time.Duration
	IsOver                  bool
}

type Bot struct {
	*tb.Bot

	Config *Config

	mu sync.Mutex

	// Chat ID => session
	sessions map[int64]*DrawSession
}

func newBot(token string, conf *Config) *Bot {
	bot, err := tb.NewBot(tb.Settings{
		Token: token,
		Poller: &tb.LongPoller{
			Timeout: 10 * time.Second,
		},
	})

	if err != nil {
		log.Fatal(err)
		return nil
	}

	return &Bot{
		Bot:      bot,
		Config:   conf,
		sessions: make(map[int64]*DrawSession),
	}
}

// handleStart
func (b *Bot) handleStart(m *tb.Message) {
	b.Send(m.Sender, "Hi, I'm a lottery bot\n\nPlease enter /help to see the usage")
}

// handleLuckyDraw creates a lottery event
func (b *Bot) handleLuckyDraw(m *tb.Message) {
	log.Println("handleLuckyDraw", m.Text)

	if m.Private() {
		b.Send(m.Sender, "can not run in a private chat")
		return
	}

	admins, err := b.AdminsOf(m.Chat)
	if err != nil {
		log.Println(err)
		return
	}

	fromAdmin := false
	for _, admin := range admins {
		if admin.User.ID == m.Sender.ID {
			fromAdmin = true
		}
	}
	if !fromAdmin {
		b.Send(m.Chat, "you are not an admin")
		return
	}

	if s, exists := b.sessions[m.Chat.ID]; exists && !s.IsOver {
		b.Send(m.Chat, b.Config.Messages.TheDrawIsAlreadyStartedAndHasNotStoppedYet)
		return
	}

	lines := strings.Split(m.Text, "\n")
	if len(lines) < 2 {
		b.Send(m.Chat, "invalid input format")
		return
	}

	var prizeEntries []PrizeEntry
	for _, line := range lines[1:] {
		inputs := strings.SplitN(line, "x", 2)
		if len(inputs) < 2 {
			b.Send(m.Chat, "invalid prize entry format")
			return
		}

		number, err := strconv.Atoi(strings.TrimSpace(inputs[0]))
		if err != nil {
			b.Send(m.Chat, "invalid quantity format")
			return
		}

		prizeEntries = append(prizeEntries, PrizeEntry{
			Name:     strings.TrimSpace(inputs[1]),
			Quantity: number,
		})
	}

	session := &DrawSession{
		Organizer:               m.Sender,
		JoinedMembers:           make(map[int]*tb.User),
		WinningMembers:          make(map[int]*tb.User),
		PrizeEntries:            prizeEntries,
		JoinDuration:            1 * time.Minute,
		PrizeAnnouncementDelay:  3 * time.Second,
		WinnerAnnouncementDelay: 3 * time.Second,
	}

	message, err := b.Send(m.Chat, format(b.Config.Messages.LuckyDrawStart, H{
		"joinDuration": session.JoinDuration.Minutes(),
	}), markdownOption)
	if err != nil {
		log.Println(err)
		return
	}

	session.Message = message

	b.mu.Lock()
	b.sessions[m.Chat.ID] = session
	b.mu.Unlock()

	go b.startDrawSession(session)
}

func (b *Bot) startDrawSession(session *DrawSession) {
	// after 10 seconds, we choose the prize
	fromTime := time.Now()
	reportTimer := time.NewTicker(1 * time.Minute)

	endC := time.After(session.JoinDuration)
WaitForJoin:
	for {
		select {
		case t := <-reportTimer.C:
			elapsed := t.Sub(fromTime).Round(time.Minute)
			timeLeft := session.JoinDuration - elapsed

			// last 3 minutes!
			if timeLeft <= 3*time.Minute {
				b.Send(session.Message.Chat, format(b.Config.Messages.TimeLeftForJoin, H{
					"timeLeft": timeLeft.Minutes(),
				}), markdownOption)
			}

		case <-endC:
			break WaitForJoin
		}
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if len(session.JoinedMembers) == 0 {
		b.Send(session.Message.Chat, format(b.Config.Messages.NoOneJoined, H{}), markdownOption)
		return
	} else if len(session.JoinedMembers) == 1 {
		b.Send(session.Message.Chat, format(b.Config.Messages.ThereIsOneMemberJoined, H{
			"numberOfMembers": len(session.JoinedMembers),
		}), markdownOption)
	} else {
		b.Send(session.Message.Chat, format(b.Config.Messages.ThereAreNMembersJoined, H{
			"numberOfMembers": len(session.JoinedMembers),
		}), markdownOption)
	}

	for k, prizeEntry := range session.PrizeEntries {
		// all members got their prizes, quit
		if len(session.JoinedMembers) == 0 {
			b.Send(session.Message.Chat, b.Config.Messages.AllMembersGotTheirPrize, markdownOption)
			break
		}

		available := prizeEntry.Quantity

		// choose winners
		for available > 0 {
			if len(session.MemberIDList) == 0 || len(session.JoinedMembers) == 0 {
				break
			}

			idx := rand.Intn(len(session.MemberIDList))
			memberID := session.MemberIDList[idx]
			member, ok := session.JoinedMembers[memberID]
			if !ok {
				continue
			}

			session.PrizeEntries[k].Winners =
				append(session.PrizeEntries[k].Winners, member)

			session.WinningMembers[member.ID] = member

			delete(session.JoinedMembers, memberID)
			available--
		}

		if prizeEntry.Quantity == 1 {
			b.Send(session.Message.Chat,
				format(b.Config.Messages.WillChooseOnePerson, H{
					"quantity": prizeEntry.Quantity,
					"prize":    prizeEntry.Name,
				}), markdownOption)
		} else {
			b.Send(session.Message.Chat,
				format(b.Config.Messages.WillChooseNumberOfPersons, H{
					"quantity": prizeEntry.Quantity,
					"prize":    prizeEntry.Name,
				}), markdownOption)
		}

		numOfWinners := len(session.PrizeEntries[k].Winners)
		for idx, winner := range session.PrizeEntries[k].Winners {
			<-time.After(session.PrizeAnnouncementDelay)

			place := translateOrdinalNumberToEnglish(numOfWinners - idx)
			b.Send(session.Message.Chat,
				format(b.Config.Messages.WinnerIs, H{
					"place":        place,
					"place_en":     place,
					"place_number": numOfWinners - idx,
					"winner":       mentionUser(winner),
				}), markdownOption)

			<-time.After(session.WinnerAnnouncementDelay)

			b.Send(session.Message.Chat, format(b.Config.Messages.NotifyWinner, H{
				"prize":     prizeEntry.Name,
				"winner":    mentionUser(winner),
				"organizer": mentionUser(session.Organizer),
			}), markdownOption)
		}
	}

	b.Send(session.Message.Chat, b.Config.Messages.TheDrawIsOver, markdownOption)
	session.IsOver = true
}

func (b *Bot) handleJoinDraw(m *tb.Message) {
	b.mu.Lock()
	session, exists := b.sessions[m.Chat.ID]
	b.mu.Unlock()

	if !exists {
		b.Send(m.Sender, b.Config.Messages.TheDrawIsNotStartedYet)
		return
	}

	if session.IsOver {
		b.Send(m.Sender, b.Config.Messages.TheDrawIsOver)
		return
	}

	session.JoinedMembers[m.Sender.ID] = m.Sender
	session.MemberIDList = append(session.MemberIDList, m.Sender.ID)
	log.Printf("adding member %s to the session", m.Sender.Username)
}

func (b *Bot) handleText(m *tb.Message) {
	log.Println("handleText", m.Text)

	if !m.IsReply() {
		return
	}

	b.mu.Lock()
	session, exists := b.sessions[m.Chat.ID]
	b.mu.Unlock()

	if !exists {
		b.Send(m.Sender, b.Config.Messages.TheDrawIsNotStartedYet)
		return
	}

	if session.IsOver {
		b.Send(m.Sender, b.Config.Messages.TheDrawIsOver)
		return
	}

	if m.ReplyTo.ID == session.Message.ID {
		session.JoinedMembers[m.Sender.ID] = m.Sender
		session.MemberIDList = append(session.MemberIDList, m.Sender.ID)
		log.Printf("adding member %s to the session", m.Sender.Username)
	}
}

func (b *Bot) handleHelp() {

}

// Command Tosscup consume optional string and reply random baubei image
func (b *Bot) handleTossCup(m *tb.Message) {
	idx := rand.Intn(len(tossCupImgs))
	b.Send(m.Chat, tossCupImgs[idx])
}

func (b *Bot) Start() {
	// bot.Handle(tb.OnText, ListenCreated)
	b.Handle("/start", b.handleStart)
	b.Handle("/luckyDraw", b.handleLuckyDraw)
	b.Handle("/joinDraw", b.handleJoinDraw)
	b.Handle("/toss", b.handleTossCup)
	//b.Handle("/help", b.handleHelp)
	b.Handle(tb.OnText, b.handleText)

	log.Println("bot started")
	b.Bot.Start()
}

func format(fmtStr string, args interface{}) string {
	var buf bytes.Buffer
	tt := template.Must(template.New("").Parse(fmtStr))
	err := tt.Execute(&buf, args)
	if err != nil {
		log.Println(err)
		return ""
	}

	return buf.String()
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config.yaml", "config file")
	flag.Parse()

	if _, err := os.Stat(".env.local"); err == nil {
		if err := godotenv.Load(".env.local"); err != nil {
			log.Fatal(err)
		}
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if len(token) == 0 {
		log.Fatal("env TELEGRAM_BOT_TOKEN is not set")
	}

	conf, err := readConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}

	bot := newBot(token, conf)
	bot.Start()
}
