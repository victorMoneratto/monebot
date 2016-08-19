package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/victormoneratto/monebot"
	"github.com/victormoneratto/monebot/util"
	"github.com/victormoneratto/telegram-bot-api"
)

var reply struct {
	asd int
}

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
			var reply struct {
				To int
				//forceReply tgbotapi.ForceReply
			}

			if update.Message == nil {
				log.Printf("Received unsupported update: %#v\n", update)
				return
			}

			message := update.Message

			log.Printf("Received: '%s' from %s\n", message.Text, message.From)

			if message.IsCommand() {
				pack, name, explicitPack := SplitCmdName(message.Command())
				if !explicitPack {
					pack, err = db.FindPack(message.Chat.ID)
					if err != nil {
						log.Println("Error finding pack:", err)
						pack = ""
					}
				}

				param := message.CommandArguments()

				switch  name {

				case "neverforget":
					fallthrough
				case "never4get":
					//if space := strings.IndexFunc(param, unicode.IsSpace()) {
					//
					//}


				case "i":
					// Show info about command
					paramSlice := SplitParams(param)
					c, err := db.FindCommand(pack, name, len(paramSlice))
					if err != nil {
						log.Printf("Error finding command '%s.%s': %s", pack, name, err)
						return
					}

					ans.Text, ans.Parse = monebot.MessageCommandInfo(c)

				default:
					// Search for a saved command
					paramSlice := SplitParams(param)
					c, err := db.FindCommand(pack, name, len(paramSlice))
					if err != nil {
						log.Printf("Error finding command %s.%s %v: %s", pack, name, param, err)
					}

					ans = c.Answer
					if c.Answer.NumParams > 0 {
						p := make([]interface{}, 0, len(paramSlice))
						for _, param := range paramSlice {
							p = append(p, param)
						}
						ans.Text = fmt.Sprintf(ans.Text, p...)
					}

					if update.Message.ReplyToMessage != nil {
						reply.To = update.Message.ReplyToMessage.MessageID
					}

					log.Printf("Answering known command from %s: %s.%s [%s]\n", update.Message.From, pack, name, param)
				}
			} else {

			}



			var send tgbotapi.Chattable
			if ans.Sticker != "" {
				sticker := tgbotapi.NewStickerShare(update.Message.Chat.ID, ans.Sticker)
				send = sticker
			} else if ans.Text != "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, ans.Text)
				msg.ParseMode = ans.Parse
				msg.ReplyToMessageID = reply.To
				msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply:true, Selective: true}
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

func SplitCmdName(c string) (pack, name string, explicit bool) {
	parts := strings.SplitN(c, ".", 2)

	switch len(parts) {
	case 1:
		pack = ""
		name = parts[0]
		explicit = false
	case 2:
		pack = parts[0]
		name = parts[1]
		explicit = true
	}

	return
}

func SplitParams(p string) []string {
	return strings.Split(p, ", ")
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
		if indexStr := submatches[len(submatches) - 1]; indexStr == "" {
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
	return regexp.MustCompile("%#?(?:\\[\\d+\\])?[^%s\\s\\[]").ReplaceAllStringFunc(s,
		func(match string) string {
			start := strings.IndexRune(match, '[')
			end := strings.IndexRune(match, ']')

			// Handle indexed verb
			if start < end {
				index, err := strconv.Atoi(match[start + 1 : end])
				if err != nil {
					return "%s"
				}
				return fmt.Sprintf("%%[%d]s", index)
			}

			return "%s"
		})
}