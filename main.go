package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Alka-devel/ruwiki-term"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

/*
Сделать ответку
"Мне придётся примкнуть к мощам иисуса чтобы это осуществить"
*/
type teacherEntry struct {
	aliases []string
	subject string
	fio     string
}

var teacherLookup = []teacherEntry{
	{[]string{"матем", "алгебра", "геометрия", "вероятность", "матеша", "геом", "алг"}, " учителя математики", "Наталья Владимировна"},
	{[]string{"русский язык", "русиш", "русский"}, " учителя русского языка", "Сергей николаевич"},
	{[]string{"английский", "англ"}, " учителя английского языка", "Наталья Игоревна (каб.11) / Ульяна Александровна (каб. 44)"},
	{[]string{"история", "ист", "общество", "общага"}, " учителя истории и обществознания", "Мария Викторовна"},
	{[]string{"физкультура", "физра", "физр"}, " учителя физкультуры", "Константин Александрович "},
	{[]string{"инф", "информатика"}, " учителя информатики", "Наталья Михайловна / Роман Николаевич"},
	{[]string{"химия", "хим"}, " учителя химии", "Ольга Дмитриевна"},
	{[]string{"биология", "клас", "био", "класс"}, " учителя биологии", "Ирина Владимировна"},
	{[]string{"география", "геог", "проект", "индив"}, " учителя географии", "Елизавета Петровна"},
	{[]string{"физ", "физика", "дура"}, " учителя физики", "Ольга Ивановна"},
	{[]string{"обж", "обзр", "безопас"}, " учителя ОБЖ", "Юрий Анатольевич"},
	{[]string{"говнюка", "пидора", "еблан"}, "", "голосуйте что сюда вставить"},
}
var (
	// ───────────── PATHS ─────────────
	path    = "schedule.json"
	claPath = "classmates.json"
	// ───────────── REGISTER ─────────────
	week    *WeekSchedule
	weekErr error
	Groups  *ClassRegistry
	grErr   error
	browser *ruwiki.Browser
	brrErr  error
	// ───────────── ARGUMENTS ─────────────
	waiter       = NewWaiter()
	rp           = false
	emojiEnabled = false
	botToken     string
)

func main() {
	fmt.Println("Start")
	flag.BoolVar(&emojiEnabled, "emojiEnabled", emojiEnabled, "Enable emoji handler")
	flag.BoolVar(&rp, "rp-coms", rp, "Enable RP handler")
	flag.StringVar(&botToken, "token", "", "Token from BotFather")
	flag.StringVar(&path, "table-path", path, "path to table with data")
	flag.Parse()
	browser, brrErr = ruwiki.StartChrome()
	if brrErr != nil {
		log.Fatal(brrErr)
	}
	defer browser.Close()

	Groups, grErr = LoadClassRegistry(claPath)
	week, weekErr = LoadWeekSchedule(path)
	if grErr != nil {
		log.Fatal(grErr)
	}
	if weekErr != nil {
		log.Fatal(weekErr)
	}

	bot, ctx := load()
	updates, _ := bot.UpdatesViaLongPolling(ctx, nil)
	bh, _ := th.NewBotHandler(bot, updates)
	defer func() { _ = bh.Stop() }()
	initComs(bh)
	_ = bh.Start()
}
func load() (*telego.Bot, context.Context) {
	ctx := context.Background()
	bot, err := telego.NewBot(botToken, telego.WithExtendedDefaultLogger(false, true, nil), telego.WithHTTPClient(&http.Client{}))
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
	return bot, ctx
}
func initComs(bh *th.BotHandler) {
	//============
	waiterCom(bh)
	reGroupCom(bh)
	startCom(bh)
	anonmsgCom(bh, 138)
	scheduleCom(bh)
	callbackHan(bh)
	fioCom(bh)
	VACUUUUMCLEANEER(bh)
	interCom(bh)
	setCom(bh)
	delDayCom(bh)
	termCom(bh)
	if rp {
		irisComs(bh)
	}
	if emojiEnabled {
		getEmojiID(bh)
	}
	//==============
	//==============
}
func delDayCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chID := tu.ID(update.Message.Chat.ID)
		args := strings.TrimPrefix(update.Message.Text, "/delDay ")
		date, err := time.Parse(dateLayout, args)
		if err != nil {
			_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, fmt.Sprintf("Неверный формат: %s", err)))
			return nil
		}
		if err := week.DeleteDay(date); err != nil {
			_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, fmt.Sprintf("Ошибка удаления дня: %s", err)))
			return nil
		}
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, "День удалён из расписания"))
		return nil
	}, th.CommandEqual("delDay"))
}
func setCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chID := tu.ID(update.Message.Chat.ID)
		args := strings.TrimPrefix(update.Message.Text, "/set ")
		day, err := ParseSchedule(strings.NewReader(args))
		if err != nil {
			mes := fmt.Sprintf("Не удалось сделать парс: %s", err)
			_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, mes))
			return nil
		}
		week.SetDay(day)
		if e := week.Save(path); e != nil {
			_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, fmt.Sprintln(e)))
			return nil
		}
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, "Расписание сохранено"))
		return nil
	}, th.CommandEqual("set"))
}
func fioCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		it := strings.TrimPrefix(strings.ToLower(update.Message.Text), "/name ")
		it = strings.TrimPrefix(it, "имя")
		it = strings.TrimPrefix(it, "учитель")
		result := "Имя: не найдено"
	search:
		for _, e := range teacherLookup {
			for _, alias := range e.aliases {
				if strings.Contains(it, alias) {
					result = fmt.Sprintf("Имя%s: %s", e.subject, e.fio)
					break search
				}
			}
		}
		_, _ = ctx.Bot().SendMessage(ctx, tu.MessageWithEntities(
			update.Message.Chat.ChatID(),
			tu.Entity(result),
			tu.Entity("😒").CustomEmoji("5424972470023104089"),
		))
		return nil
	}, th.Or(th.CommandEqual("name"), th.TextPrefix("имя"), th.TextPrefix("Имя"), th.TextPrefix("учитель"), th.TextPrefix("Учитель")))
}
func anonmsgCom(bh *th.BotHandler, threadId int) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		if threadId != -1 {
			parm := &telego.CopyMessageParams{
				ChatID:          tu.ID(-1004443888902),
				MessageThreadID: threadId,
				FromChatID:      update.Message.Chat.ChatID(),
				MessageID:       update.Message.MessageID,
			}
			_, _ = ctx.Bot().CopyMessage(ctx, parm)
			_, _ = ctx.Bot().SendMessage(ctx, tu.Message(update.Message.Chat.ChatID(), "Скоро.. скоро.."))
		}
		return nil
	}, th.CommandEqual("anonmsg"))
}
func scheduleCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		args := ":empty"
		if update.Message.Chat.Type == "private" {
			if !Check(update.Message.Chat.ID) {
				_, e := ctx.Bot().SendMessage(ctx, tu.MessageWithEntities(
					update.Message.Chat.ChatID(),
					tu.Entity("Выбирай группу!"),
				).WithReplyMarkup(tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("Я в ИТ!").WithCallbackData("it").WithIconCustomEmojiID("5312259896677259918").WithStyle(telego.ButtonStyleSuccess),
						tu.InlineKeyboardButton("Я в СЭ!").WithCallbackData("se").WithIconCustomEmojiID("5204280252737537692").WithStyle(telego.ButtonStylePrimary),
					),
					tu.InlineKeyboardRow(tu.InlineKeyboardButton("Оставить как есть").WithCallbackData("nothing")),
				)))
				if e != nil {
					fmt.Println(e)
				}
				return nil
			}
			g, oki := Groups.GetGroup(update.Message.From.ID)
			args = fmt.Sprintf(":%d", g)
			fmt.Println("аргу", args, "окии", oki)
		}
		forceSch := func() error {
			f := false
			nDay := 0
			now := time.Now()
			for i := now; !f && nDay < 14; i = i.Add(24 * time.Hour) {
				day, ok := week.GetDay(i)
				if !ok {
					nDay++
					continue
				}
				if nDay == 0 && dayFinished(day, now) {
					nDay++
					continue
				}
				f = true
				schImg(ctx, update.Message.GetChat().ChatID(), day, f, Empty)
				return nil
			}
			if !f {
				_, _ = ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(update.Message.Chat.ID), "Ближайшее расписание не найдено"))
				return nil
			}
			return nil
		}

		if strings.Contains(update.Message.Text, "да") {
			return forceSch()
		}
		sp := strings.TrimPrefix(strings.ToLower(update.Message.Text), "/schedule")
		sp = strings.TrimPrefix(sp, "расписание")

		btn1 := tu.InlineKeyboardButton("Да").WithIconCustomEmojiID("5388749682216280524").WithStyle("success")
		btn2 := tu.InlineKeyboardButton("Нет").WithIconCustomEmojiID("5217944373362174845").WithStyle("Danger").WithCallbackData("nothing")

		if sp != "" {
			switch {
			case strings.Contains(sp, "ближайшее"):
				return forceSch()
			default:
				_, _ = ctx.Bot().SendMessage(ctx, tu.Message(
					tu.ID(update.Message.Chat.ID),
					fmt.Sprintf("%s, показать расписание?", update.Message.From.FirstName),
				).WithReplyMarkup(tu.InlineKeyboard(tu.InlineKeyboardRow(btn1.WithCallbackData(fmt.Sprintf("showScheduleImg:%s:t%s", strings.TrimSpace(sp), args)), btn2))))
				return nil
			}
		}
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(
			tu.ID(update.Message.Chat.ID),
			fmt.Sprintf("%s, показать расписание?", update.Message.From.FirstName),
		).WithReplyMarkup(tu.InlineKeyboard(tu.InlineKeyboardRow(btn1.WithCallbackData("showSchedule"), btn2))))
		return nil
	}, th.Or(th.CommandEqual("schedule"), th.TextPrefix("Расписание"), th.TextPrefix("расписание")))
}
func getEmojiID(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, message telego.Update) error {
		for _, e := range message.Message.Entities {
			if e.Type == telego.EntityTypeCustomEmoji {
				_, _ = ctx.Bot().SendMessage(ctx, tu.Message(
					tu.ID(message.Message.Chat.ID),
					e.CustomEmojiID,
				))
			}
		}
		return nil
	}, th.Or(th.TextContains("emoji"), th.TextContains("Emoji"), th.TextContains("емоджи"), th.TextContains("Емоджи"), th.TextContains("эмоджи"), th.TextContains("Эмоджи")))
}

func interCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		day, ok := week.GetDay(time.Now())
		if !ok {
			day = ScheduleDay{}
		}
		_, _ = ctx.Bot().SendMessage(ctx, Status(time.Now()).Params(update.Message.Chat.ChatID(), day))
		return nil
	}, th.Or(th.CommandEqual("interruption"), th.TextContains("Перемена"), th.TextContains("перемена")))
}
func startCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(
			tu.ID(update.Message.Chat.ID),
			fmt.Sprintf("Привет, %s! Если вдруг у тебя появились идеи или хочешь сообщить об ошибке, пиши @ThisNameReallyExists ☺️", update.Message.From.FirstName),
		).WithReplyMarkup(tu.Keyboard(
			tu.KeyboardRow(
				tu.KeyboardButton("Расписание"),
				tu.KeyboardButton("Перемена"),
			),
			tu.KeyboardRow(tu.KeyboardButton("Поменять группу")),
		).WithResizeKeyboard()))
		if !Check(update.Message.Chat.ID) {
			ctx.Bot().SendMessage(ctx, tu.MessageWithEntities(
				update.Message.Chat.ChatID(),
				tu.Entity("Также тебе надо сделать выбор в какой ты группе!"),
			).WithReplyMarkup(tu.InlineKeyboard(
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("Я в ИТ!").WithCallbackData("it").WithIconCustomEmojiID("5312259896677259918").WithStyle(telego.ButtonStyleSuccess),
					tu.InlineKeyboardButton("Я в СЭ!").WithCallbackData("se").WithIconCustomEmojiID("5204280252737537692").WithStyle(telego.ButtonStylePrimary),
				),
			)))
		}
		return nil
	}, th.CommandEqual("start"))
}
func reGroupCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		if update.Message.Chat.Type != "private" {
			return nil
		}
		_, e := ctx.Bot().SendMessage(ctx, tu.MessageWithEntities(
			update.Message.Chat.ChatID(),
			tu.Entity("Выбирай группу!"),
		).WithReplyMarkup(tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Я в ИТ!").WithCallbackData("it").WithIconCustomEmojiID("5312259896677259918").WithStyle(telego.ButtonStyleSuccess),
				tu.InlineKeyboardButton("Я в СЭ!").WithCallbackData("se").WithIconCustomEmojiID("5204280252737537692").WithStyle(telego.ButtonStylePrimary),
			),
			tu.InlineKeyboardRow(tu.InlineKeyboardButton("Оставить как есть").WithCallbackData("nothing")),
		)))
		if e != nil {
			fmt.Println(e)
		}
		return nil
	}, th.Or(
		th.CommandEqual("change"),
		th.TextEqualFold("поменять группу"),
		th.TextEqualFold("группа"),
	))
}
func waiterCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		if update.Message != nil {
			waiter.Dispatch(update.Message.Chat.ChatID(), update)
		}
		if update.CallbackQuery != nil {
			waiter.Dispatch(update.CallbackQuery.Message.GetChat().ChatID(), update)
		}
		ctx.Next(update)
		return nil
	}, th.Any())
}
func VACUUUUMCLEANEER(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		if update.Message.From.ID != 5613804018 {
			return nil
		}
		chid := update.Message.GetChat().ChatID()
		threadID := update.Message.MessageThreadID
		highest := update.Message.MessageID
		botID := ctx.Bot().ID()
		logChat := tu.ID(5613804018)

		fmt.Println("botID =", botID) // сверим на всякий случай

		ctx.Bot().DeleteMessage(ctx, tu.Delete(chid, update.Message.MessageID))

		var deleted int
		for id := 1; id < highest; id++ {
			fwd, err := ctx.Bot().ForwardMessage(ctx, &telego.ForwardMessageParams{
				ChatID:          logChat,
				MessageThreadID: threadID,
				FromChatID:      chid,
				MessageID:       id,
			})
			if err != nil {
				continue
			}

			// ВРЕМЕННЫЙ ДЕБАГ
			fmt.Printf("id=%d from.ID=%d from.IsBot=%v forwardOrigin=%#v\n",
				id, fwd.From.ID, fwd.From.IsBot, fwd.ForwardOrigin)

			isFromBot := false
			if origin, ok := fwd.ForwardOrigin.(*telego.MessageOriginUser); ok {
				isFromBot = origin.SenderUser.ID == botID
				fmt.Println("  -> MessageOriginUser, sender.ID =", origin.SenderUser.ID, "match:", isFromBot)
			} else {
				fmt.Printf("  -> НЕ MessageOriginUser, конкретный тип: %T\n", fwd.ForwardOrigin)
			}

			_ = ctx.Bot().DeleteMessage(ctx, tu.Delete(logChat, fwd.MessageID))

			if isFromBot {
				if err := ctx.Bot().DeleteMessage(ctx, tu.Delete(chid, id)); err == nil {
					deleted++
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		fmt.Println("удалено сообщений бота:", deleted)
		return nil
	}, th.Or(
		th.CommandEqual("clean"),
		th.TextEqualFold("очистка"),
	))
}
func termCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		if update.Message.Chat.Type != "private" {
			ctx.Bot().DeleteMessage(ctx, tu.Delete(
				update.Message.Chat.ChatID(),
				update.Message.MessageID))
			return nil
		}
		ctx.Bot().SendMessage(ctx, tu.Message(update.Message.Chat.ChatID(), "Ищу значение."))
		word := strings.TrimSpace(strings.ToLower(update.Message.Text))
		word = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(word,
			"значение"),
			"термин"),
			"/term")
		_, te, e := ruwiki.SearchTerm(word)
		if e != nil {
			log.Fatal(e)
		}
		ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(update.Message.From.ID), te))
		return e
	}, th.Or(
		th.CommandEqual("term"),
		th.TextPrefix("значение"), th.TextPrefix("Значение"),
		th.TextPrefix("термин"), th.TextPrefix("Термин"),
	))
}
