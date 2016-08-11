package monebot

import (
	"fmt"
	"github.com/victormoneratto/monebot/util"
	"strings"
)

func MessageSavedCommand(c Command) (Text, Parse string) {
	Text = fmt.Sprintf("Saved command *%s* `(with %d parameters)`",
		util.EscapeMarkdown(c.FullName()), c.Answer.NumParams)
	Parse = ParseMarkdown

	return
}

func MessageCommandInfo(c Command) (Text, Parse string){
	year, month, day := c.Time.Date()

	creator := util.EscapeMarkdown(c.Creator)
	if !strings.HasPrefix(c.Creator, "@") {
		creator = fmt.Sprintf("`%s`", creator)
	}

	Text = fmt.Sprintf(
		"*%s* `(with %d parameters)`\n"+
		"_%s_\n\n"+
		"*Last updated by* %s *on* `%d/%d/%d`",
		util.EscapeMarkdown(c.FullName()), c.Answer.NumParams,
		util.EscapeMarkdown(c.Answer.Text),
		creator, year, month, day)

	Parse = ParseMarkdown

	return
}

func MessageMissingName() (Text, Parse string) {
	Text = "Please, send me the command's name"
	Parse = ""

	return
}

func MessageMissingContent() (Text, Parse string) {
	Text = "Please, send me the content for the command"
	Parse = ""

	return
}