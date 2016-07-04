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

	"github.com/victormoneratto/monebot"
	"github.com/victormoneratto/monebot/util"
	"github.com/victormoneratto/telegram-bot-api"
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
	db, err := monebot.NewDatabase(util.MustGetEnv("DATABASE_CONN_URI"))
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
		go func() {
			var ans monebot.Answer // Set a value for Answer to reply on chat
			var reply int
			var forceReply tgbotapi.ForceReply
			if update.Message == nil {
				log.Printf("Received unsupported update: %#v\n", update)
				return
			}

			log.Printf("Received: '%s' from %s\n", update.Message.Text, update.Message.From)

			if !update.Message.IsCommand() {
				state, err := db.FindState(update.Message.Chat.ID, update.Message.From.String())
				if err != nil {
					log.Println("Error finding state:", err)
					return
				}

				if state.Waiting.ForCommand {
					pack, name, param, _ := Parse(update.Message.Text, update.Message.Chat.ID, db)
					if state.Waiting.Pack == "" {
						state.Waiting.Pack = pack
					}
					if state.Waiting.Command == "" {
						state.Waiting.Command = name
					} else {
						param = update.Message.Text
					}

					if state.Waiting.Command != "" && param != "" {
						c, err := SaveCommand(state.Waiting.Pack, state.Waiting.Command,
							update.Message.Text, update.Message.From.String(), db)
						if err != nil {
							log.Println("Error saving command:", err)
							return
						}
						ans.Text = fmt.Sprintf("Saved command *%s* `(with %d parameters)`",
							EscapeMarkdown(c.FullName()), c.Answer.NumParams)
						ans.Parse = monebot.ParseMarkdown

						err = db.RemoveState(update.Message.Chat.ID, update.Message.From.String())
						if err != nil {
							log.Println("Error removing state:", err)
							return
						}
					} else {
						db.UpsertState(
							monebot.State{
								Chat:       update.Message.Chat.ID,
								User:       update.Message.From.String(),
								LastUpdate: time.Now(),
								Waiting: monebot.WaitingState{
									ForCommand: true,
									Pack:       pack,
									Command:    name}})

						if name != "" && param == "" {
							forceReply = tgbotapi.ForceReply{ForceReply: true, Selective: true}
							ans.Text = "Please, send me the content for the command"
							reply = update.Message.MessageID
						}
					}
				}
			} else {

				// Parse command
				pack, name, param, params := Parse(update.Message.Text, update.Message.Chat.ID, db)

				if name == "" {
					log.Println("Unsupported message text", update.Message.Text)
					return
				}

				switch name {

				case "neverforget":
					fallthrough
				case "never4get":

					pack, name, param, _ := Parse(param, update.Message.Chat.ID, db)

					if param == "" {
						forceReply = tgbotapi.ForceReply{ForceReply: true, Selective: true}
						s := monebot.State{
							Chat: update.Message.Chat.ID,
							User: update.Message.From.String(),
							Waiting: monebot.WaitingState{
								ForCommand: true,
								Pack:       pack,
								Command:    name}}
						err := db.UpsertState(s) // Save that we're waiting for command
						if err != nil {
							log.Println("Error upserting state:", err)
							return
						}
						if name == "" {
							ans.Text = "Please, send me a name for the command"
						} else {
							ans.Text = "Please, send me a content for the command"
						}
						reply = update.Message.MessageID
						break
					} else {
						// Update or Insert a command
						c, err := SaveCommand(pack, name, param, update.Message.From.String(), db)
						if err != nil {
							log.Println("Error saving command:", err)
							return
						}
						ans.Text = fmt.Sprintf("Saved command *%s* `(with %d parameters)`",
							EscapeMarkdown(c.FullName()), c.Answer.NumParams)
						ans.Parse = monebot.ParseMarkdown
						return
					}

				case "i":
					// Show info about command
					pack, name, _, params := Parse(param, update.Message.Chat.ID, db)
					c, err := db.FindCommand(pack, name, len(params))
					if err != nil {
						log.Println("Error finding command:", err)
						return
					}

					year, month, day := c.Time.Date()
					creator := EscapeMarkdown(c.Creator)
					if !strings.HasPrefix(c.Creator, "@") {
						creator = fmt.Sprintf("`%s`", creator)
					}
					ans.Text = fmt.Sprintf(
						"*%s* `(with %d parameters)`\n"+
							"_%s_\n\n"+
							"*Last updated by* %s *on* `%d/%d/%d`",
						EscapeMarkdown(c.FullName()), c.Answer.NumParams,
						EscapeMarkdown(c.Answer.Text),
						creator, year, month, day)

					ans.Parse = monebot.ParseMarkdown

				default:
					// Search for a saved command
					c, err := db.FindCommand(pack, name, len(params))
					if err != nil {
						log.Printf("Error finding command %s.%s %v: %s", pack, name, param, err)
					}

					ans = c.Answer
					if c.Answer.NumParams > 0 {
						p := make([]interface{}, 0, len(params))
						for _, param := range params {
							p = append(p, param)
						}
						ans.Text = fmt.Sprintf(ans.Text, p...)
					}

					if update.Message.ReplyToMessage != nil {
						reply = update.Message.ReplyToMessage.MessageID
					}
				}

				if ans.Text == "" {
					return
				}

				log.Printf("Handled command %s: %s.%s %s\n", update.Message.From, pack, name, param)
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, ans.Text)
			msg.ParseMode = ans.Parse
			msg.ReplyToMessageID = reply
			msg.ReplyMarkup = forceReply

			_, err = bot.Send(msg)
			if err != nil {
				log.Println("Error sending message:", err)
			}

		}()
	}
}

// SaveCommand updates or inserts a command
func SaveCommand(pack, name, param, creator string, db *monebot.Database) (monebot.Command, error) {
	var c monebot.Command
	var err error

	c.Pack = pack
	c.Name = name
	c.Answer.Text = RemoveUnsupportedVerbs(param)
	c.Answer.NumParams = CountVerbs(c.Answer.Text)
	c.Creator = creator
	c.Time = time.Now()

	err = db.UpsertCommand(c)
	if err != nil {
		return c, err
	}

	return c, nil
}

func EscapeMarkdown(s string) string {
	// TODO a regex to escape links as well
	return regexp.MustCompile("[\\*_`]").ReplaceAllStringFunc(s,
		func(match string) string {
			return "\\" + match
		})
}

// CountVerbs returns the number of string verbs (%s, %[1]s...)
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
func Parse(message string, chat int64, db *monebot.Database) (pack, name string, param string, params []string) {
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
		var err error
		pack, err = db.FindPack(chat)
		if err != nil {
			pack = ""
		}
	}

	if len(param) > 0 {
		params = strings.Split(param, ", ")
	}

	return
}
