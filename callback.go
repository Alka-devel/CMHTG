package main

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func deleteQueryMessage(ctx *th.Context, query telego.CallbackQuery) {
	if err := ctx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
		ChatID:    query.Message.GetChat().ChatID(),
		MessageID: query.Message.GetMessageID(),
	}); err != nil {
		log.Println("не удалось удалить сообщение:", err)
	}
}

var groupByCode = map[int]Group{1: SocEco, 2: InfTec}

func showScheduleImg(ctx *th.Context, query telego.CallbackQuery) bool {
	dsa := strings.Split(strings.TrimPrefix(query.Data, "showScheduleImg:"), ":")
	if len(dsa) < 3 || dsa[0] == "" {
		return false
	}
	chID := query.Message.GetChat().ChatID()
	day, ok := week.GetDayByDate(dsa[0])
	deleteQueryMessage(ctx, query)
	if !ok {
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, fmt.Sprintf("Расписание на %s не найдено.", dsa[0])))
		return true
	}
	code, _ := strconv.Atoi(dsa[2])
	schImg(ctx, chID, day, dsa[1] == "t", groupByCode[code])
	return true
}

func callbackHan(bh *th.BotHandler) {
	bh.HandleCallbackQuery(func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if r := recover(); r != nil {
				log.Println("callback panic:", r)
				ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID))
			}
		}()
		chID := query.Message.GetChat().ChatID()
		switch {
		case strings.HasPrefix(query.Data, "schedule"):
			scheduleCallback(ctx, query, strings.TrimPrefix(query.Data, "schedule"))
		case strings.HasPrefix(query.Data, "announcement"):
			announcementCallback(ctx, query, strings.TrimPrefix(query.Data, "announcement"))
		case strings.HasPrefix(query.Data, "members"):
			membersCallback(ctx, query, strings.TrimPrefix(query.Data, "members"))
		}
		if strings.HasPrefix(query.Data, "showScheduleImg") {
			_ = ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID))
			if showScheduleImg(ctx, query) {
				return nil
			}
		}
		switch query.Data {
		case "showSchedule":
			deleteQueryMessage(ctx, query)
			day, ok := week.GetDay(time.Now())
			if !ok {
				_, err := ctx.Bot().SendMessage(ctx, tu.Message(chID, "Расписание на сегодня не найдено."))
				return err
			}
			var grr = Empty
			grr, _ = Groups.GetGroup(query.From.ID)
			schImg(ctx, chID, day, false, grr)
		case "nothing":
			deleteQueryMessage(ctx, query)
		case "empty":
			tabl(Empty, query, ctx, false)
		case "empty:t":
			tabl(Empty, query, ctx, true)
		case "it":
			tabl(InfTec, query, ctx, false)
		case "se":
			tabl(SocEco, query, ctx, false)
		case "it:t":
			tabl(InfTec, query, ctx, true)
		case "se:t":
			tabl(SocEco, query, ctx, true)
		case "nothing:t":
			cab(query, ctx)
		case "cab":
			cab(query, ctx)
		case "settings":
			dood, ok := Groups.Get(query.From.ID)
			if !ok {
				ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID))
				return nil
			}
			seti(query, ctx, dood)
		case "settings:f":
			Groups.Update(query.From.ID, func(c *Classmate) { c.Configured = false })
			dood, ok := Groups.Get(query.From.ID)
			if !ok {
				ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID))
				return nil
			}
			seti(query, ctx, dood)
		case "info":
			inf(query, ctx)
		case "change":
			change(query, ctx)
		case "right_name":
			Groups.Update(query.From.ID, func(c *Classmate) {
				if n, err := cleanName(c.Name); err == nil {
					c.Name = n
				}
			})
			nextPage(query, ctx)
		case "wrong_name":
			wrName(query, ctx)
		case "inv:no":
			Groups.Update(query.From.ID, func(c *Classmate) { c.Innovations = false })
			nextPage(query, ctx)
		case "inv:yes":
			Groups.Update(query.From.ID, func(c *Classmate) { c.Innovations = true })
			nextPage(query, ctx)
		case "dut:no":
			Groups.Update(query.From.ID, func(c *Classmate) { c.Duty = false })
			nextPage(query, ctx)
		case "dut:yes":
			Groups.Update(query.From.ID, func(c *Classmate) { c.Duty = true })
			nextPage(query, ctx)
		case "dr":
			Groups.Update(query.From.ID, func(c *Classmate) { c.Announcement = "all" })
			nextPage(query, ctx)
		case "dr:self":
			Groups.Update(query.From.ID, func(c *Classmate) { c.Announcement = "self" })
			nextPage(query, ctx)
		case "dr:other":
			Groups.Update(query.From.ID, func(c *Classmate) { c.Announcement = "other" })
			nextPage(query, ctx)
		case "dr:non":
			Groups.Update(query.From.ID, func(c *Classmate) { c.Announcement = "non" })
			nextPage(query, ctx)
		case "admin":
			adminpanel(query, ctx, 0)
		}
		ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID))
		return nil
	}, th.AnyCallbackQueryWithMessage())

}
func adminpanel(query telego.CallbackQuery, ctx *th.Context, state int) {
	parts := []tu.MessageEntityCollection{
		tu.Entity("😇").CustomEmoji("5199597179311788893"),
		tu.Entity(" Выберите действие.").Bold(),
	}
	var rows [][]telego.InlineKeyboardButton
	switch state {
	case 0:
		parts = parts[:len(parts)-1]
		parts = append(parts, tu.Entity(" Выберите категорию для управления.").Bold())
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Расписание").WithCallbackData("schedule").WithStyle("success"),     //1
			tu.InlineKeyboardButton("Оповещения").WithCallbackData("announcement").WithStyle("success"), //2
			tu.InlineKeyboardButton("Участники").WithCallbackData("members").WithStyle("success")))      //3
	case 1:
		parts = append(parts, tu.Entity("\nКатегория: Расписание").Bold())
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Удалить день").WithStyle("danger").WithCallbackData("schedule:del"),
			tu.InlineKeyboardButton("Изменить").WithStyle("primary").WithCallbackData("schedule:edit"),
			tu.InlineKeyboardButton("Установить").WithStyle("success").WithCallbackData("schedule:set"),
		))
	case 2:
		parts = append(parts, tu.Entity("\nКатегория: Оповещения").Bold())
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Черновики").WithCallbackData("announcement:drafts"),
			tu.InlineKeyboardButton("Публикация").WithStyle("primary").WithCallbackData("announcement:publish"),
			tu.InlineKeyboardButton("Создать").WithStyle("success").WithCallbackData("announcement:make"),
		))
	case 3:
		parts = append(parts, tu.Entity("\nКатегория: Участники").Bold())
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Удалить").WithStyle("danger").WithCallbackData("members:del"),
			tu.InlineKeyboardButton("Создать принудительно").WithStyle("primary").WithCallbackData("members:make"),
			tu.InlineKeyboardButton("Редактировать").WithStyle("primary").WithCallbackData("members:edit"),
		))
	}
	if state > 0 {
		rows = append(rows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад к панели").WithCallbackData("admin")))
	}
	txt, enty := tu.MessageEntities(parts...)
	ctx.Bot().EditMessageText(ctx, tu.EditMessageText(
		query.Message.GetChat().ChatID(),
		query.Message.GetMessageID(),
		txt,
	).WithEntities(enty...).WithReplyMarkup(tu.InlineKeyboard(rows...)))
}
func scheduleCallback(ctx *th.Context, query telego.CallbackQuery, s string) {
	if s == "" {
		adminpanel(query, ctx, 1)
		return
	}
	parts := strings.Split(s, ":")
	if len(parts) > 0 {
		parts = slices.Delete(parts, 0, 1)
	}
	switch parts[0] {
	case "del":
		delDayInDays := func(txt string) {
			dat, er := time.Parse(dateLayout, txt)
			delErr := week.DeleteDay(dat)
			if delErr != nil || er != nil {
				ctx.Bot().SendMessage(ctx, tu.Messagef(query.Message.GetChat().ChatID(), "Ошибка: %s", delErr))
				ctx.Bot().EditMessageText(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Произошла ошибка.").WithReplyMarkup(tu.InlineKeyboard(tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад к панели").WithCallbackData("admin").WithStyle("danger")))))
				return
			}
			ctx.Bot().SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), "День удалён из расписания."))
		}
		ctx.Bot().EditMessageText(ctx,
			tu.EditMessageText(query.Message.GetChat().ChatID(),
				query.Message.GetMessageID(),
				`<tg-emoji emoji-id="5199597179311788893">😇</tg-emoji><b> Напиши дату дня, которого хочешь удалить или выбери из списка</b>`).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("Вчерашний").WithCallbackData("schedule:del:yesterday").WithStyle("primary"),
						tu.InlineKeyboardButton("Сегодняшний").WithCallbackData("schedule:del:today").WithStyle("danger"),
						tu.InlineKeyboardButton("Завтрашний").WithCallbackData("schedule:del:tomorrow").WithStyle("primary"),
					),
					tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад к панели").WithCallbackData("admin").WithStyle("success")),
				)))
		if len(parts) > 1 && parts[1] != "" {
			switch parts[1] {
			case "yesterday":
				delDayInDays(time.Now().AddDate(0, 0, -1).Format(dateLayout))
				return
			case "today":
				delDayInDays(time.Now().Format(dateLayout))
				return
			case "tomorrow":
				delDayInDays(time.Now().AddDate(0, 0, 1).Format(dateLayout))
				return
			}
		}
		if upd, err := waiter.WaitForMessage(ctx, query.Message.GetChat().ChatID()); err == nil && upd.Message != nil {
			delDayInDays(upd.Message.Text)
			return
		}
	case "edit":
	// /*
	// * СДЕЛАТЬ!!
	// *
	// *
	// *
	// *
	//  */
	case "set":
		// /*
		// * СДЕЛАТЬ!!
		// *
		// *
		// *
		// *
		//  */
	}

}
func announcementCallback(ctx *th.Context, query telego.CallbackQuery, s string) {
	if s == "" {
		adminpanel(query, ctx, 2)
		return
	}
}
func membersCallback(ctx *th.Context, query telego.CallbackQuery, s string) {
	if s == "" {
		adminpanel(query, ctx, 3)
		return
	}
	parts := strings.Split(s, ":")
	if len(parts) > 0 {
		parts = slices.Delete(parts, 0, 1)
	}
	switch parts[0] {
	case "del":
		var (
			id    int64
			dood  Classmate
			found bool
			idErr error
		)
		hasID := len(parts) > 1 && parts[1] != ""
		if hasID {
			if id, idErr = strconv.ParseInt(parts[1], 10, 64); idErr == nil {
				dood, found = Groups.Get(id)
			}
		}

		chatID, msgID := query.Message.GetChat().ChatID(), query.Message.GetMessageID()
		const emoji = `<tg-emoji emoji-id="5199597179311788893">😇</tg-emoji><b> `

		switch {
		case hasID && len(parts) > 2 && parts[2] == "t":
			err := idErr
			if err == nil && !found {
				err = fmt.Errorf("пользователь %d не найден", id)
			}
			if err == nil {
				err = Groups.Delete(id)
			}
			if err != nil {
				ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID,
					emoji+`Произошла ошибка: `+html.EscapeString(err.Error())+`</b>`,
				).WithParseMode("HTML"))
				return
			}
			ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID,
				emoji+`Пользователь (`+html.EscapeString(displayName(id, dood))+`) успешно удалён</b>`,
			).WithParseMode("HTML"))

		case hasID:
			if !found {
				return
			}
			ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID,
				emoji+`Ты точно уверен(-а), что хочешь удалить этого (`+html.EscapeString(displayName(id, dood))+`) человека?</b>`,
			).WithParseMode("HTML").WithReplyMarkup(tu.InlineKeyboard(
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("Да").WithStyle("danger").WithCallbackData(fmt.Sprintf("members:del:%d:t", id)),
					tu.InlineKeyboardButton("Нет").WithStyle("success").WithCallbackData("members"),
				),
			)))

		default:
			var baty []telego.InlineKeyboardButton
			for _, uid := range Groups.FirstIDs(4) {
				if u, ok := Groups.Get(uid); ok {
					baty = append(baty, tu.InlineKeyboardButton(displayName(uid, u)).
						WithCallbackData(fmt.Sprintf("members:del:%d", uid)).
						WithStyle("primary"))
				}
			}
			ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID,
				emoji+`Выбери человека из списка или напиши его username для удаления</b>`,
			).WithParseMode("HTML").WithReplyMarkup(tu.InlineKeyboard(
				tu.InlineKeyboardRow(baty...),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад к панели").WithCallbackData("members").WithStyle("success")),
			)))
		}
		return
	case "make":
	case "edit":
	}
}
func nextPage(query telego.CallbackQuery, ctx *th.Context) {
	Groups.Update(query.From.ID, func(c *Classmate) { c.SettingsPage += 1 })
	dood, ok := Groups.Get(query.From.ID)
	if !ok {
		return
	}
	seti(query, ctx, dood)
}
func wrName(query telego.CallbackQuery, ctx *th.Context) {
	chat := tu.ID(query.From.ID)
	ctx.Bot().DeleteMessage(ctx, tu.Delete(chat, query.Message.GetMessageID()))
	ctx.Bot().SendMessage(ctx, tu.Message(chat, "<b>Понял! Жду твоё настоящее имя!</b>").WithParseMode("html"))

	for attempt := 0; attempt < 3; attempt++ {
		upd, err := waiter.WaitForMessage(ctx, chat)
		if err != nil || upd.Message == nil {
			return
		}
		name, nerr := cleanName(upd.Message.Text)
		if nerr != nil {
			ctx.Bot().SendMessage(ctx, tu.Message(chat, "Не подходит: "+nerr.Error()+"\nНапиши настоящее имя ещё раз."))
			continue
		}
		Groups.Update(query.From.ID, func(c *Classmate) { c.Name = name })
		if dood, ok := Groups.Get(query.From.ID); ok {
			seti(query, ctx, dood)
		}
		return
	}
	ctx.Bot().SendMessage(ctx, tu.Message(chat, "Не получилось. Нажми «Настройки» в личном кабинете и попробуй снова."))
}
func seti(query telego.CallbackQuery, ctx *th.Context, dood Classmate) {
	num := dood.SettingsPage
	parts := []tu.MessageEntityCollection{
		tu.Entity("⚙").CustomEmoji("5877260593903177342"),
		tu.Entityf(" Быстрая настройка (%d/5):\n", num+1).Bold(),
	}
	var rows [][]telego.InlineKeyboardButton
	if dood.Configured && dood.SettingsPage == 0 {
		parts = nil
		parts = append(parts, tu.Entity("💬").CustomEmoji("5931614414351372818"),
			tu.Entity(" Все настройки уже выставлены, хотите начать всё заново?").Bold())
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Нет").WithCallbackData("nothing:t").WithStyle("primary"),
			tu.InlineKeyboardButton("Да").WithCallbackData("settings:f").WithStyle("primary"),
		))
		num = 8
	}
	switch num {
	case 0:
		if _, err := cleanName(dood.Name); err != nil {
			wrName(query, ctx)
			return
		}
		parts = append(parts, tu.Entity("💬").CustomEmoji("5936017305585586269"),
			tu.Entityf(" Твоё настоящее имя - %s?", dood.Name).Bold())
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Нет").WithCallbackData("wrong_name").WithStyle("danger"),
			tu.InlineKeyboardButton("Да").WithCallbackData("right_name").WithStyle("success"),
		))
	case 1:
		parts = append(parts, tu.Entity("💬").CustomEmoji("5891243564309942507"),
			tu.Entity(" Выбери о чьих днях рождения тебе придёт оповещение?").Bold())
		rows = append(rows,
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Получать все").WithCallbackData("dr").WithStyle("success"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Своё").WithCallbackData("dr:self").WithStyle("primary"),
				tu.InlineKeyboardButton("Одноклассников").WithCallbackData("dr:other").WithStyle("primary"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Ничего не получать").WithCallbackData("dr:non").WithStyle("danger"),
			),
		)
	case 2:
		parts = append(parts, tu.Entity("💬").CustomEmoji("5906995262378741881"),
			tu.Entity("  Укажите свой день рождения в формате ДД.ММ.ГГГГ").Bold())
	case 3:
		parts = append(parts, tu.Entity("💬").CustomEmoji("5931614414351372818"),
			tu.Entity(" Сообщать о нововведениях в бота?").Bold())
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Нет").WithCallbackData("inv:no").WithStyle("primary"),
			tu.InlineKeyboardButton("Да").WithCallbackData("inv:yes").WithStyle("primary"),
		))
	case 4:
		parts = append(parts, tu.Entity("💬").CustomEmoji("5891243564309942507"),
			tu.Entity(" Сообщать о днях, когда Вы дежурите?").Bold())
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Нет").WithCallbackData("dut:no").WithStyle("primary"),
			tu.InlineKeyboardButton("Да").WithCallbackData("dut:yes").WithStyle("primary"),
		))
	case 5:
		parts = nil
		parts = append(parts, tu.Entity("💬").CustomEmoji("5951665890079544884"),
			tu.Entity(" Спасибо за настройку!").Bold())
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Назад в личный кабинет").WithCallbackData("cab").WithStyle("danger").WithIconCustomEmojiID("5877629862306385808")))
		Groups.Update(query.From.ID, func(c *Classmate) {
			c.Configured = true
			c.SettingsPage = 0
			c.Notify = c.Announcement == "all" && c.Duty && c.Innovations
		})
	}
	txt, enty := tu.MessageEntities(parts...)
	var ed = tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), txt).WithEntities(enty...)
	if len(rows) > 0 {
		ed = ed.WithReplyMarkup(tu.InlineKeyboard(rows...))
	}
	_, er := ctx.Bot().EditMessageText(ctx, ed)
	if er != nil {
		ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(query.From.ID), txt).WithEntities(enty...).WithReplyMarkup(tu.InlineKeyboard(rows...)))
	}
	if num == 2 {
		upd, er := waiter.WaitForMessage(ctx, tu.ID(query.From.ID))
		if er != nil || upd.Message == nil {
			return
		}
		dat, dErr := parseBirthday(upd.Message.Text)
		if dErr != nil {
			ctx.Bot().EditMessageText(ctx, tu.EditMessageText(tu.ID(query.From.ID), query.Message.GetMessageID(), "Не могу распознать дату."))
			seti(query, ctx, dood)
			return
		}
		Groups.Update(query.From.ID, func(c *Classmate) {
			c.Birthday = dat
		})
		nextPage(query, ctx)
	}
}
func cab(query telego.CallbackQuery, ctx *th.Context) {
	grr, ok := Groups.GetGroup(query.From.ID)
	if !ok {
		return
	}
	noti, ok := Groups.Get(query.From.ID)
	if !ok {
		return
	}
	notik := "Включены"
	if !noti.Notify {
		notik = "Выключены"
	}
	txt, entities := tu.MessageEntities(
		tu.Entity("👤").CustomEmoji("5879770735999717115"), tu.Entity(" Личный кабинет").Bold(), tu.Entity("\n"),
		tu.Entity("🆔").CustomEmoji("5927118708873892465"), tu.Entity(" Ваш ID: ").Bold(), tu.Entityf("%d\n", query.From.ID),
		tu.Entity("📁").CustomEmoji("5877332341331857066"), tu.Entity(" Твоя группа: ").Bold(), tu.Entity(grr.RuString()), tu.Entity("\n"),
		tu.Entity("🔊").CustomEmoji("5890997763331591703"), tu.Entity(" Уведомления: ").Bold(), tu.Entity(notik),
	)
	keyb := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Все команды").WithIconCustomEmojiID("5884510167986343350").WithCallbackData("info").WithStyle("primary"),
			tu.InlineKeyboardButton("Поменять группу").WithIconCustomEmojiID("5883997877172179131").WithCallbackData("change").WithStyle("primary"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Настройки").WithIconCustomEmojiID("5877260593903177342").WithCallbackData("settings").WithStyle("success"),
		),
	)
	ctx.Bot().EditMessageText(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), txt).WithEntities(entities...).WithReplyMarkup(keyb))

}
func tabl(g Group, query telego.CallbackQuery, ctx *th.Context, b bool) {
	Groups.Update(query.From.ID, func(c *Classmate) {
		c.Group = g
		if c.Name == "" {
			if n, err := cleanName(query.From.FirstName); err == nil {
				c.Name = n
			}
		}
		if cah, erdr := ctx.Bot().GetChat(ctx, &telego.GetChatParams{ChatID: tu.ID(query.From.ID)}); erdr == nil && cah != nil && cah.Birthdate != nil {
			c.Birthday.Day = cah.Birthdate.Day
			c.Birthday.Month = cah.Birthdate.Month
			c.SettingsPage = 0
			if cah.Birthdate.Year != 0 {
				c.Birthday.Year = cah.Birthdate.Year
			}
		}
	})
	var txt string
	switch g {
	case InfTec:
		txt = "Успешно установлена группа ИТ\nОтправь /start для большей настройки"
	case SocEco:
		txt = "Успешно установлена группа СЭ\nОтправь /start для большей настройки"
	case Empty:
		txt = "Успешно установлена группа хз\nОтправь /start для большей настройки"
	}
	var ed = tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), txt)
	if b {
		ed = ed.WithReplyMarkup(tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Назад в личный кабинет").WithCallbackData("cab").WithStyle("danger").WithIconCustomEmojiID("5877629862306385808")))).WithText(strings.TrimSuffix(txt, "\nОтправь /start для большей настройки"))
	}
	ctx.Bot().EditMessageText(ctx, ed)
}
func change(query telego.CallbackQuery, ctx *th.Context) {
	ctx.Bot().EditMessageText(ctx, tu.EditMessageText(
		query.Message.GetChat().ChatID(),
		query.Message.GetMessageID(),
		"Выбирай группу!",
	).WithReplyMarkup(changeKeyb(true)))
}
func inf(query telego.CallbackQuery, ctx *th.Context) {
	txt, enty := allComParam()
	keyb := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Назад").WithIconCustomEmojiID("5877629862306385808").WithCallbackData("cab").WithStyle("danger"),
		))
	ctx.Bot().EditMessageText(ctx, tu.EditMessageText(
		query.Message.GetChat().ChatID(),
		query.Message.GetMessageID(),
		txt,
	).WithEntities(enty...).WithReplyMarkup(keyb))
}
func schImg(ctx *th.Context, chID telego.ChatID, d ScheduleDay, force bool, grr Group) {
	st := Status(time.Now())
	if len(d.Entries) == 0 {
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, d.String()))
		return
	}
	if st.Finished && !force {
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, st.String()))
		return
	}
	currentIndex := -1
	if st.IsLesson {
		currentIndex = st.LessonIndex + 1
	}
	img, err := RenderScheduleImage(d, currentIndex, fmtDur(st.TimeLeft), DefaultScale, grr)
	if err != nil {
		log.Println("рендер расписания:", err)
		return
	}
	png, err := EncodePNG(img)
	if err != nil {
		log.Println("encode png:", err)
		return
	}
	_, _ = ctx.Bot().SendPhoto(ctx, &telego.SendPhotoParams{
		ChatID: chID,
		Photo:  tu.FileFromReader(bytes.NewReader(png), "schedule.png"),
	})
}
