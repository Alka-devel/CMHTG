package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	// {[]string{"говнюка", "пидора", "еблан"}, "", "голосуйте что сюда вставить"},
}
var (
	// ───────────── PATHS ─────────────
	tabPath = "schedule.json"
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
	ownerID      int64
)

// ───────────── QUIET LOGGER ─────────────
type quietLogger struct{}

func (quietLogger) Debugf(string, ...any) {}
func (quietLogger) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if strings.Contains(msg, "context canceled") {
		return
	}
	log.Println("ERROR", msg)
}

//───────────── QUIET LOGGER ─────────────

func main() {
	fmt.Println("Start")
	flag.BoolVar(&emojiEnabled, "emojiEnabled", emojiEnabled, "Enable emoji handler")
	flag.BoolVar(&rp, "rp-coms", rp, "Enable RP handler")
	flag.StringVar(&botToken, "token", botToken, "Token from BotFather")
	flag.StringVar(&tabPath, "table-path", tabPath, "path to table with data")
	flag.StringVar(&claPath, "clsmts-table-path", claPath, "path to table with classmates")
	flag.Int64Var(&ownerID, "owner-tg-id", ownerID, "Telegram ID of owner of bot")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	browser, brrErr = ruwiki.StartChrome()
	if brrErr != nil {
		fmt.Println(brrErr)
	} else {
		defer browser.Close()
	}

	Groups, grErr = LoadClassRegistry(claPath)
	if grErr != nil {
		log.Fatal(grErr)
	}
	week, weekErr = LoadWeekSchedule(tabPath)
	if weekErr != nil {
		log.Fatal(weekErr)
	}

	bot := load()
	updates, _ := bot.UpdatesViaLongPolling(ctx, nil)
	bh, _ := th.NewBotHandler(bot, updates)
	initComs(bh)
	startScheduler(ctx, bot, 25*time.Minute)
	_ = bh.Start()
	_ = bh.Stop()
	if err := Groups.Save(claPath); err != nil {
		log.Println("сохранение реестра:", err)
	}
	if err := week.Save(tabPath); err != nil {
		log.Println("сохранение расписания:", err)
	}
}

func load() *telego.Bot {
	bot, err := telego.NewBot(botToken, telego.WithLogger(quietLogger{}), telego.WithHTTPClient(&http.Client{}))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return bot
}
func initComs(bh *th.BotHandler) {
	//============
	waiterCom(bh)
	reGroupCom(bh)
	startCom(bh)
	anonmsgCom(bh, 138)
	infoCom(bh)
	scheduleCom(bh)
	callbackHan(bh)
	fioCom(bh)
	VACUUUUMCLEANEER(bh)
	gdzCom(bh)
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
		args := strings.TrimPrefix(strings.ToLower(update.Message.Text), "/delday ")
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
	}, th.CommandEqual("delday"))
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
		if e := week.Save(tabPath); e != nil {
			_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, fmt.Sprintln(e)))
			return nil
		}
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, "Расписание сохранено"))
		return nil
	}, th.CommandEqual("set"))
}
func fioCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		fields := strings.Fields(update.Message.Text)
		if len(fields) > 1 {
			fields = fields[1:]
		} else {
			fields = []string{""}
		}
		result := "Имя: не найдено\nФормат: /name *предмет*\nПример: имя алгебра"
	search:
		for _, e := range teacherLookup {
			for _, alias := range e.aliases {
				if strings.Contains(fields[0], alias) {
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
			g, _ := Groups.GetGroup(update.Message.From.ID)
			args = fmt.Sprintf(":%d", g)
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
				g, _ := Groups.GetGroup(update.Message.From.ID)
				schImg(ctx, update.Message.GetChat().ChatID(), day, f, g)
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
		ctx.Bot().SendMessage(ctx, tu.MessageWithEntities(
			tu.ID(update.Message.From.ID),
			tu.Entity("✈️").CustomEmoji("5875465628285931233"), tu.Entityf(" %s, добро пожаловать!\n", update.Message.From.FirstName), tu.Entity("\n"),
			tu.Entity("💬").CustomEmoji("5884510167986343350"), tu.Entity(" Узнать все команды - /info\n"),
			tu.Entity("📢").CustomEmoji("5994378304751145264"), tu.Entity(" Сообщить об ошибке - "), tu.Entity("@ThisNameReallyExists").TextLink("http://t.me/ThisNameReallyExists"), tu.Entity("\n"),
			tu.Entity("⚙").CustomEmoji("5877260593903177342"), tu.Entity(" Настройки уведомлений - /announcement"),
		).WithReplyMarkup(tu.InlineKeyboard(
			tu.InlineKeyboardRow(tu.InlineKeyboardButton("Личный кабинет").WithIconCustomEmojiID("5879770735999717115").WithCallbackData("cab").WithStyle("primary")),
		)))
		return nil
	}, th.CommandEqual("start"))
}
func allComParam() (string, []telego.MessageEntity) {
	return tu.MessageEntities(
		tu.Entity("🏷").CustomEmoji("5854776233950188167"), tu.Entity(" Список всех команд:").Bold(), tu.Entity("\n"),
		tu.Entity("\n"),
		tu.Entity("🗒").CustomEmoji("5877597667231534929"), tu.Entity(" /schedule - Покажет расписание на сегодня, если уроки ещё идут\n"),
		tu.Entity("🔄").CustomEmoji("5778202206922608769"), tu.Entity(" /interruption - Отобразит время до урока или перемены\n"),
		tu.Entity("🏷").CustomEmoji("5987802868734760945"), tu.Entity(" /name - Подскажет имя учителя\n"),
		tu.Entity("🖋").CustomEmoji("5883997877172179131"), tu.Entity(" /change - Поменяет группу, в которой находишься\n"),
		tu.Entity("🖼").CustomEmoji("5775949822993371030"), tu.Entity(" /gdz - Сможет отправить гдз по предметам из списка\n"),
		tu.Entity("📢").CustomEmoji("5771695636411847302"), tu.Entity(" /rep - Сообщить об ошибке\n"),
	)
}
func infoCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		txt, entities := allComParam()
		_, err := ctx.Bot().SendMessage(ctx,
			tu.Message(
				tu.ID(update.Message.Chat.ID),
				txt,
			).WithEntities(entities...),
		)
		return err
	}, th.CommandEqual("info"))
}
func changeKeyb(b bool) *telego.InlineKeyboardMarkup {
	keyb := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Я в ИТ!").WithCallbackData("it:t").WithIconCustomEmojiID("5312259896677259918").WithStyle(telego.ButtonStyleSuccess),
			tu.InlineKeyboardButton("Я в СЭ!").WithCallbackData("se:t").WithIconCustomEmojiID("5204280252737537692").WithStyle(telego.ButtonStylePrimary),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Оставить как есть").WithCallbackData("nothing:t")),
	)
	if !b {
		keyb = tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Я в ИТ!").WithCallbackData("it").WithIconCustomEmojiID("5312259896677259918").WithStyle(telego.ButtonStyleSuccess),
				tu.InlineKeyboardButton("Я в СЭ!").WithCallbackData("se").WithIconCustomEmojiID("5204280252737537692").WithStyle(telego.ButtonStylePrimary),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Назад").WithIconCustomEmojiID("5877629862306385808").WithCallbackData("cab").WithStyle("danger"),
			),
		)
	}
	return keyb
}
func reGroupCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		if update.Message.Chat.Type != "private" {
			return nil
		}

		_, e := ctx.Bot().SendMessage(ctx, tu.MessageWithEntities(
			update.Message.Chat.ChatID(),
			tu.Entity("Выбирай группу!"),
		).WithReplyMarkup(changeKeyb(false)))
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
		if update.Message.From.ID != ownerID {
			return nil
		}
		chid := update.Message.GetChat().ChatID()
		threadID := update.Message.MessageThreadID
		highest := update.Message.MessageID
		botID := ctx.Bot().ID()
		logChat := tu.ID(ownerID)

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
		m, er := ctx.Bot().SendMessage(ctx, tu.Message(update.Message.Chat.ChatID(), "Ищу значение."))
		if er != nil {
			return er
		}
		word := strings.TrimSpace(strings.ToLower(update.Message.Text))
		word = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(word,
			"значение"),
			"термин"),
			"/term")
		txt := "Произошла непредвиденная ошибка"
		_, te, e := ruwiki.SearchTerm(word)
		if e != nil {
			log.Fatal(e)
		} else {
			txt = te
		}
		ctx.Bot().EditMessageText(ctx, tu.EditMessageText(update.Message.Chat.ChatID(), m.MessageID, txt))
		return e
	}, th.Or(
		th.CommandEqual("term"),
		th.TextPrefix("значение"), th.TextPrefix("Значение"),
		th.TextPrefix("термин"), th.TextPrefix("Термин"),
	))
}
