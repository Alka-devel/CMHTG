package main

import (
	"fmt"
	"strings"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var irisReactions = map[string]string{
	"67":            "отсиксевенил",
	"оттэдабаёнить": "оттэдабаёнил",
	"отэдабаёнить":  "оттэдабаёнил",
	"оттэдабаенить": "оттэдабаёнил",
	"отэдабаенить":  "оттэдабаёнил",
	"убить":         "убил",
	"закопать":      "закопал",
	"урыть":         "урыл",
	"похоронить":    "похоронил",
	"обнять":        "обнял",
	"поцеловать":    "поцеловал",
	"чмокнуть":      "чмокнул",
	"погладить":     "погладил",
	"могнуть":       "моггнул",
	"моггнуть":      "моггнул",
	"ударить":       "ударил",
	"пнуть":         "пнул",
	"укусить":       "укусил",
	"толкнуть":      "толкнул",
	"утешить":       "утешил",
	"успокоить":     "успокоил",
	"разбудить":     "разбудил",
	"простить":      "простил",
	"предать":       "предал",
	"благословить":  "благословил",
	"проклясть":     "проклял",
}

func irisComs(bh *th.BotHandler) {
	rpComs(bh)
	pinComs(bh)
}
func rpComs(bh *th.BotHandler) {
	predicates := make([]th.Predicate, 0, len(irisReactions))
	for phrase := range irisReactions {
		predicates = append(predicates, th.TextEqualFold(phrase))
	}
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		sendMes := func(s string) {
			if update.Message.ReplyToMessage == nil {
				return
			}
			member, err := ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
				ChatID: update.Message.Chat.ChatID(),
				UserID: update.Message.ReplyToMessage.From.ID,
			})
			if err != nil {
				fmt.Println(err)
				return
			}
			name := update.Message.ReplyToMessage.From.FirstName
			switch m := member.(type) {
			case *telego.ChatMemberAdministrator:
				if m.CustomTitle != "" {
					name = m.CustomTitle
				}
			case *telego.ChatMemberOwner:
				if m.CustomTitle != "" {
					name = m.CustomTitle
				}
			}
			_, _ = ctx.Bot().SendMessage(ctx, tu.MessageWithEntities(
				update.Message.Chat.ChatID(),
				tu.Entity(update.Message.From.FirstName),
				tu.Entity(s),
				tu.Entity(name),
			))
		}

		text := strings.ToLower(update.Message.Text)
		if action, ok := irisReactions[text]; ok {
			sendMes(fmt.Sprint(" ", action, " "))
		}
		return nil
	}, th.Or(predicates...))
}
func pinComs(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		ctx.Bot().SendMessage(ctx, tu.Message(
			update.Message.Chat.ChatID(),
			"ПОНГ",
		))
		return nil
	}, th.TextEqualFold("пинг"))
}
