package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var (
	path         = "schedule.json"
	claPath      = "classmates.json"
	week         *WeekSchedule
	weekErr      error
	Groups       *ClassRegistry
	grErr        error
	emojiEnabled = flag.Bool("emojiEnabled", false, "Enable emoji handler")
	botToken     = ""
	waiter       = NewWaiter()
)

func main() {
	fmt.Println("Start")
	flag.StringVar(&botToken, "token", "", "Token from BotFather")
	flag.StringVar(&path, "table-path", path, "path to table with data")
	flag.Parse()
	Groups, grErr = LoadClassRegistry(claPath)
	week, weekErr = LoadWeekSchedule(path)
	if grErr != nil {
		fmt.Println(grErr)
	}
	if weekErr != nil {
		fmt.Println(weekErr)
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
		os.Exit(1)
	}
	return bot, ctx
}
func initComs(bh *th.BotHandler) {
	//============
	waiterCom(bh)
	startCom(bh)
	anonmsgCom(bh, 138)
	scheduleCom(bh)
	irisComs(bh)
	callbackHan(bh)
	fioCom(bh)
	interCom(bh)
	setCom(bh)
	delDayCom(bh)
	if *emojiEnabled {
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
func irisComs(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		sendMes := func(s string) {
			if update.Message.ReplyToMessage == nil {
				_, _ = ctx.Bot().SendMessage(ctx, tu.Message(
					update.Message.Chat.ChatID(),
					"Нужно ответить на чьё-то сообщение, чтобы это сработало",
				))
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
			name := update.Message.ReplyToMessage.From.FirstName // фолбэк по умолчанию
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
		switch update.Message.Text {
		case "67":
			sendMes(" отсиксевенил ")
		case "оттэдабаёнить":
			sendMes(" оттэдабаёнил ")
		case "отэдабаёнить":
			sendMes(" оттэдабаёнил ")
		case "оттэдабаенить":
			sendMes(" оттэдабаёнил ")
		case "отэдабаенить":
			sendMes(" оттэдабаёнил ")
		case "убить":
			sendMes(" убил ")
		case "закопать":
			sendMes(" закопал ")
		case "урыть":
			sendMes(" урыл ")
		case "похоронить":
			sendMes(" похоронил ")
		case "обнять":
			sendMes(" обнял ")
		case "поцеловать":
			sendMes(" поцеловал ")
		case "чмокнуть":
			sendMes(" чмокнул ")
		case "погладить":
			sendMes(" погладил ")
		case "могнуть":
			sendMes(" моггнул ")
		case "моггнуть":
			sendMes(" моггнул ")
		}
		return nil
	}, th.Or(
		th.TextContains("67"),
		th.TextEqualFold("оттэдабаёнить"),
		th.TextEqualFold("отэдабаёнить"),
		th.TextEqualFold("оттэдабаенить"),
		th.TextEqualFold("отэдабаенить"),
		th.TextEqualFold("убить"),
		th.TextEqualFold("закопать"),
		th.TextEqualFold("урыть"),
		th.TextEqualFold("похоронить"),
		th.TextEqualFold("обнять"),
		th.TextEqualFold("поцеловать"),
		th.TextEqualFold("чмокнуть"),
		th.TextEqualFold("погладить"),
		th.TextEqualFold("могнуть"),
		th.TextEqualFold("моггнуть"),
		// th.TextEqualFold(""),
	))
}
func fioCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		var it string
		it = strings.TrimPrefix(strings.ToLower(update.Message.Text), "/name ")
		it = strings.TrimPrefix(it, "имя")
		it = strings.TrimPrefix(it, "учитель")
		_, _ = ctx.Bot().SendMessage(ctx, tu.MessageWithEntities(
			update.Message.Chat.ChatID(),
			tu.Entity(fio(it)),
			tu.Entity("😒").CustomEmoji("5424972470023104089"),
		))
		return nil
	}, th.Or(th.CommandEqual("name"), th.TextPrefix("имя"), th.TextPrefix("Имя"), th.TextPrefix("учитель"), th.TextPrefix("Учитель")))
}
func fio(it string) string {
	mes := "Имя"
	var te string
	var fio string
	switch {
	case conca(it, "матем", "алгебра", "геометрия", "вероятность", "матеша", "геом", "алг"):
		te = " учителя математики"
		fio = "Наталья Владимировна"
	case conca(it, "русский язык", "русиш", "русский"):
		te = " учителя русского языка"
		fio = "Сергей николаевич"
	case conca(it, "английский", "англ"):
		te = " учителя английского языка"
		fio = "Наталья Игоревна (каб.11) / Ульяна Александровна (каб. 44)"
	case conca(it, "история", "ист", "общество", "общага"):
		te = " учителя истории и обществознания"
		fio = "Мария Викторовна"
	case conca(it, "физкультура", "физра", "физр"):
		te = " учителя физкультуры"
		fio = "Константин Александрович "
	case conca(it, "инф", "информатика"):
		te = " учителя информатики"
		fio = "Наталья Михайловна / Роман Николаевич"
	case conca(it, "химия", "хим"):
		te = " учителя химии"
		fio = "Ольга Дмитриевна"
	case conca(it, "биология", "клас", "био", "класс"):
		te = " учителя биологии"
		fio = "Ирина Владимировна"
	case conca(it, "география", "геог", "проект", "индив"):
		te = " учителя географии"
		fio = "Елизавета Петровна"
	case conca(it, "физ", "физика", "дура"):
		te = " учителя физики"
		fio = "Ольга Ивановна"
	case conca(it, "обж", "обзр", "безопас"):
		te = " учителя ОБЖ"
		fio = "Юрий Анатольевич"
	case conca(it, "говнюка", "пидора", "еблан"):
		te = ""
		fio = "голосуйте что сюда вставить"
	default:
		te = ""
		fio = "не найдено"
	}
	return fmt.Sprintf("%s%s: %s", mes, te, fio)
}
func conca(i string, l ...string) bool {
	I := strings.ToLower(i)
	for _, b := range l {
		if strings.Contains(I, strings.ToLower(b)) {
			return true
		}
	}
	return false
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
				schImg(ctx, update.Message.GetChat().ChatID(), day, f)
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

		btn1 := tu.InlineKeyboardButton("Да").WithCallbackData("showSchedule")
		btn2 := tu.InlineKeyboardButton("Нет").WithCallbackData("nothing")
		btn1.IconCustomEmojiID = "5388749682216280524"
		btn1.Style = telego.ButtonStyleSuccess
		btn2.IconCustomEmojiID = "5217944373362174845"
		btn2.Style = telego.ButtonStyleDanger

		if sp != "" {
			switch {
			case strings.Contains(sp, " ближайшее"):
				return forceSch()
			default:
				_, _ = ctx.Bot().SendMessage(ctx, tu.Message(
					tu.ID(update.Message.Chat.ID),
					fmt.Sprintf("%s, показать расписание?", update.Message.From.FirstName),
				).WithReplyMarkup(tu.InlineKeyboard(tu.InlineKeyboardRow(btn1.WithCallbackData(fmt.Sprintf("showScheduleImg:%s:t", strings.TrimSpace(sp))), btn2.WithCallbackData("nothing")))))
				return nil
			}
		}
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(
			tu.ID(update.Message.Chat.ID),
			fmt.Sprintf("%s, показать расписание?", update.Message.From.FirstName),
		).WithReplyMarkup(tu.InlineKeyboard(tu.InlineKeyboardRow(btn1, btn2))))
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
		))
		if Check(update.Message.Chat.ID) {
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
