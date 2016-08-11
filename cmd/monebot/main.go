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
	bot, err := tgbotapi.NewBotAPI(util.MustGetenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		panic(err)
	}

	// Connect to database
	db, err := monebot.NewDatabase(util.MustGetenv("DATABASE_CONN_URI"))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Listen for updates
	updates, err := bot.GetUpdatesChan(tgbotapi.NewUpdate(0))
	if err != nil {
		panic(err)
	}

	log.Printf("@%s started\n", bot.Self.UserName)

	// Indefinitely loop for updates
	for update := range updates {
		go func() {
			var ans monebot.Answer
			var replyTo int
			var forceReply tgbotapi.ForceReply
			if update.Message == nil {
				log.Printf("Received unsupported update: %#v\n", update)
				return
			}

			log.Printf("Received: '%s' from %s\n", update.Message.Text, update.Message.From)

			if !update.Message.IsCommand() {
				state, err := db.FindState(update.Message.Chat.ID, update.Message.From.ID)
				if err != nil {
					log.Println("Error finding state:", err)
					return
				}

				if state.Waiting.ForCommand {
					cmd := monebot.Command{}
					pack, name, param, _ := Parse(update.Message.Text, update.Message.Chat.ID, db)

					if sticker := update.Message.Sticker; sticker != nil {
						cmd.Answer = NewStickerAnswer(sticker.FileID)
					} else if state.Waiting.Command != "" {
						// We already had a name, the whole message is the text content
						cmd.Answer = NewTextAnswer(update.Message.Text)
					} else {
						cmd.Answer = NewTextAnswer(param)
					}

					if state.Waiting.Pack != "" {
						cmd.Pack = state.Waiting.Pack
					} else {
						cmd.Pack = pack
					}

					if state.Waiting.Command != "" {
						cmd.Name = state.Waiting.Command
					} else {
						cmd.Name = name
					}

					if cmd.Name != "" {
						var c monebot.Command
						if cmd.Answer.Text != "" || cmd.Answer.Sticker != "" {
							// NOTE: This should receive the monebot.Command, probably
							c, err = SaveCommand(cmd.Pack, cmd.Name,
								update.Message.From.String(), cmd.Answer, db)
							if err != nil {
								log.Println("Error saving command:", err)
								return
							}

							ans.Text, ans.Parse = monebot.MessageSavedCommand(c)

							err = db.RemoveState(update.Message.Chat.ID, update.Message.From.ID)
							if err != nil {
								log.Println("Error removing state:", err)
								return
							}
						} else {
							s := monebot.NewWaitingState(update.Message.Chat.ID,
								update.Message.From.ID,
								monebot.WaitingState{
									ForCommand: true,
									Pack:       cmd.Pack,
									Command:    cmd.Name})
							db.UpsertState(s)

							replyTo = update.Message.MessageID
							forceReply = tgbotapi.ForceReply{ForceReply: true, Selective: true}
							ans.Text, ans.Parse = monebot.MessageMissingContent()
						}
					} else {
						//NOTE: This is the same code for the missing content case, refactor
						s := monebot.NewWaitingState(update.Message.Chat.ID,
							update.Message.From.ID,
							monebot.WaitingState{
								ForCommand: true,
								Pack:       pack})
						db.UpsertState(s)

						replyTo = update.Message.MessageID
						forceReply = tgbotapi.ForceReply{ForceReply: true, Selective: true}
						ans.Text, ans.Parse = monebot.MessageMissingName()
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
						// NOTE: This is almost the same as lines above, REFACTOR IMMEDIATELY
						replyTo = update.Message.MessageID
						forceReply = tgbotapi.ForceReply{ForceReply: true, Selective: true}
						s := monebot.NewWaitingState(update.Message.Chat.ID,
							update.Message.From.ID,
							monebot.WaitingState{
								ForCommand: true,
								Pack:       pack,
								Command:    name})

						// Save that we're waiting for command
						err := db.UpsertState(s)
						if err != nil {
							log.Println("Error upserting state:", err)
							return
						}
						if name == "" {
							ans.Text, ans.Parse = monebot.MessageMissingName()
						} else {
							ans.Text, ans.Parse = monebot.MessageMissingContent()
						}
					} else {
						//NOTE: disabled to avoid even more code duplication for now
						// Update or Insert a command
						//c, err := SaveCommand(pack, name, NewTextAnswer(param), update.Message.From.String(), db)
						//if err != nil {
						//	log.Println("Error saving command:", err)
						//	return
						//}
						//ans.Text, ans.Parse = monebot.MessageSavedCommand(c)
					}

				case "i":
					// Show info about command
					pack, name, _, params := Parse(param, update.Message.Chat.ID, db)
					c, err := db.FindCommand(pack, name, len(params))
					if err != nil {
						log.Printf("Error finding command '%s.%s': %s", pack, name, err)
						return
					}

					ans.Text, ans.Parse = monebot.MessageCommandInfo(c)

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
						replyTo = update.Message.ReplyToMessage.MessageID
					}
				}

				log.Printf("Answered known command from %s: %s.%s [%s]\n", update.Message.From, pack, name, param)
			}

			var send tgbotapi.Chattable
			if ans.Sticker != "" {
				sticker := tgbotapi.NewStickerShare(update.Message.Chat.ID, ans.Sticker)
				send = sticker
			} else if ans.Text != "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, ans.Text)
				msg.ParseMode = ans.Parse
				msg.ReplyToMessageID = replyTo
				msg.ReplyMarkup = forceReply
				send = msg
			}

			if send != nil {
				_, err = bot.Send(send)
				if err != nil {
					log.Println("Error sending message:", err)
				}
			}
		}()
	}
}

func NewTextAnswer(text string) monebot.Answer {
	text = RemoveUnsupportedVerbs(text)
	return monebot.Answer{Text: text, NumParams: CountVerbs(text)}
}

func NewStickerAnswer(sticker string) monebot.Answer {
	return monebot.Answer{Sticker: sticker}
}

// saveCommand updates or inserts a command
func SaveCommand(pack, name, creator string, ans monebot.Answer, db *monebot.Database) (monebot.Command, error) {
	var c monebot.Command
	var err error

	c.Pack = pack
	c.Name = name
	c.Answer = ans
	c.Creator = creator
	c.Time = time.Now()

	err = db.UpsertCommand(c)
	if err != nil {
		return c, err
	}

	return c, nil
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
// trying to replace most unsupported Printf verbs (%d, %[1]v, %#v etc.)
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
		// NOTE: Maybe this should be extracted to some aliasing function
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
