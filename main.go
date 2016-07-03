package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/victormoneratto/monebot/util"
	"github.com/victormoneratto/telegram-bot-api"
	"gopkg.in/mgo.v2"
)

func main() {
	// Setup logging for heroku
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	// Connect to telegram
	bot, err := tgbotapi.NewBotAPI(util.MustGetEnv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		panic(err)
	}

	// Connect to database
	db, err := NewDatabase(util.MustGetEnv("DATABASE_CONN_URI"))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Listen for updates
	updates, err := bot.GetUpdatesChan(tgbotapi.NewUpdate(0))
	if err != nil {
		panic(err)
	}

	log.Println("@"+bot.Self.UserName, "started")

	// Indefinitely loop for updates
	for update := range updates {
		if update.Message == nil {
			log.Printf("Received unsupported update: %#v\n", update)
			continue
		}

		log.Printf("Received: '%s' from %s\n", update.Message.Text, update.Message.From)

		if !update.Message.IsCommand() {
			continue
		}

		// Parse command
		needsPack, pack, name, param := Parse(update.Message.Text)
		if needsPack {
			// Message doesn't explicit a pack, get the default for the chat
			pack, err = db.FindPack(update.Message.Chat.ID)
			if err != nil && err != mgo.ErrNotFound {
				log.Println("Error finding pack:", err)
			}
		}

		if name == "" {
			log.Println("Unsupported message text", update.Message.Text)
			continue
		}

		var ans Answer // Set a value for Answer to reply on chat
		switch name {

		// Update or Insert a command
		case "neverforget":
			fallthrough
		case "never4get":
			err := SaveCommand(param, update.Message.From.String(), update.Message.Chat.ID, db)
			if err != nil {
				log.Println("Error saving command:", err)
				continue
			}

		// Search for a saved command
		default:
			ans, err = FindCommand(pack, name, param, db)
			if err != nil {
				log.Printf("Error finding command %s.%s %v: %s", pack, name, param, err)
			}
		}

		if ans.Text == "" {
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, ans.Text)
		if update.Message.ReplyToMessage != nil {
			msg.ReplyToMessageID = update.Message.ReplyToMessage.MessageID
		}

		_, err = bot.Send(msg)
		if err != nil {
			log.Println("Error sending message:", err)
			continue
		}

		log.Printf("Answered %s: %s.%s %s\n", update.Message.From, pack, name, param)
	}
}

// FindCommand returns the answer from the saved command
func FindCommand(pack, name, param string, db *Database) (Answer, error) {
	var ans Answer
	params := strings.Split(param, ", ")
	if params[0] == "" {
		params = nil
	}
	c, err := db.FindCommand(pack, name, len(params))
	if err != nil {
		return ans, err
	}

	ans = c.Answer
	if ans.NumParams > 0 {
		p := make([]interface{}, 0, len(params))
		for _, param := range params {
			p = append(p, param)
		}
		ans.Text = fmt.Sprintf(ans.Text, p...)
	}

	return ans, nil
}

// SaveCommand updates or inserts a command
func SaveCommand(param, creator string, chat int64, db *Database) error {
	var c Command
	var needsPack bool
	var err error

	needsPack, c.Pack, c.Name, param = Parse(param)
	c.Answer.Text = RemoveUnsupportedVerbs(param)
	c.Answer.NumParams = CountVerbs(c.Answer.Text)
	c.Creator = creator
	c.Time = time.Now()
	if needsPack {
		c.Pack, err = db.FindPack(chat)
		if err != nil && err != mgo.ErrNotFound {
			return err
		}
	}

	err = db.UpsertCommand(c)
	if err != nil {
		return err
	}

	return nil
}

// CountVerbs retuns the number of string verbs (%s, %[1]s...)
// taking into consideration indexed and non-indexed verbs
func CountVerbs(s string) int {
	matches := regexp.MustCompile("%(?:\\[(\\d+)\\])?s").FindAllStringSubmatch(s, -1)
	var numNotIndexed, maxIndex int
	for _, submatches := range matches {
		if indexStr := submatches[len(submatches)-1]; indexStr == "" {
			numNotIndexed++
		} else {
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				numNotIndexed++
				continue
			}
			if index > maxIndex {
				maxIndex = index
			}
		}
	}
	if maxIndex > numNotIndexed {
		return maxIndex
	}
	return numNotIndexed
}

// RemoveUnsupportedVerbs returns a cleaner version of a format string,
// trying to replace most unsupported verbs
func RemoveUnsupportedVerbs(s string) string {
	// TODO this regex doesn't match %#s
	return regexp.MustCompile("%#?(?:\\[\\d+\\])?[^%s\\s\\[]").ReplaceAllStringFunc(s,
		func(match string) string {
			start := strings.IndexRune(match, '[')
			end := strings.IndexRune(match, ']')

			// Handle indexed verb
			if start < end {
				index, err := strconv.Atoi(match[start+1 : end])
				if err != nil {
					return "%s"
				}
				return fmt.Sprintf("%%[%d]s", index)
			}

			return "%s"
		})
}

// Parse returns command information from message
func Parse(message string) (needsPack bool, pack, name string, param string) {
	// Remove heading "/"
	message = strings.TrimPrefix(message, "/")

	var fullName string
	// There's no strings.SplitFunc, we'll separate the first word manually
	space := strings.IndexFunc(message, unicode.IsSpace)
	if space != -1 {
		fullName = message[:space] // Name is just the first word
		param = strings.TrimSpace(message[space:])
	} else {
		fullName = message // Name is the whole message
		param = ""
	}

	// Separate fullname into "<pack>.<name>", where "<pack>." is optional
	nameParts := strings.SplitN(fullName, ".", 2)
	if len(nameParts) == 2 {
		// Pack is explicit in message (<pack>.<name>)
		pack = strings.TrimSpace(nameParts[0])
		if pack == "default" {
			pack = ""
		}
		name = strings.TrimSpace(nameParts[1])
	} else {
		// No pack was specified
		name = nameParts[0]
		needsPack = true
	}

	return
}
