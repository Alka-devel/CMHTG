package main

import (
	"bytes"
	"context"
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

type adminDlg struct {
	ctx    *th.Context
	chatID telego.ChatID
	msgID  int
	back   []telego.InlineKeyboardButton
}

func newAdminDlg(ctx *th.Context, query telego.CallbackQuery, backData string) *adminDlg {
	ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID))
	return &adminDlg{
		ctx:    ctx,
		chatID: query.Message.GetChat().ChatID(),
		msgID:  query.Message.GetMessageID(),
		back:   tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад к панели").WithCallbackData(backData).WithStyle("success")),
	}
}
func (d *adminDlg) show(text string, rows ...[]telego.InlineKeyboardButton) {
	rows = append(rows, d.back)
	d.ctx.Bot().EditMessageText(d.ctx, tu.EditMessageText(d.chatID, d.msgID,
		`<tg-emoji emoji-id="5818920837645867167">😇</tg-emoji><b> `+text+`</b>`).
		WithParseMode(telego.ModeHTML).
		WithReplyMarkup(tu.InlineKeyboard(rows...)))
}

func (d *adminDlg) askMsgUntil(redraw func(errLine string), validate func(*telego.Message) error) *telego.Message {
	redraw("")
	for range 3 {
		wctx, cancel := context.WithTimeout(d.ctx, 2*time.Minute)
		upd, err := waiter.WaitForMessage(wctx, d.chatID)
		cancel()
		if err != nil || upd.Message == nil {
			return nil
		}
		m := upd.Message
		d.ctx.Bot().DeleteMessage(d.ctx, tu.Delete(d.chatID, m.MessageID))
		if err := validate(m); err == nil {
			return m
		} else {
			echo := []rune(m.Text)
			if len(echo) > 60 {
				echo = append(echo[:60], '…')
			}
			redraw(fmt.Sprintf("«%s» не подходит: %s\n", html.EscapeString(string(echo)), html.EscapeString(err.Error())))
		}
	}
	redraw("Не получилось, используй кнопки ниже.\n")
	return nil
}

func (d *adminDlg) askUntil(redraw func(errLine string), validate func(string) error) string {
	m := d.askMsgUntil(redraw, func(m *telego.Message) error { return validate(strings.TrimSpace(m.Text)) })
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m.Text)
}

func (d *adminDlg) askDate(prefix string) string {
	kb := tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Вчера").WithCallbackData(prefix+":yesterday").WithStyle("primary"),
		tu.InlineKeyboardButton("Сегодня").WithCallbackData(prefix+":today").WithStyle("success"),
		tu.InlineKeyboardButton("Завтра").WithCallbackData(prefix+":tomorrow").WithStyle("primary"),
	)
	return d.askUntil(func(errLine string) {
		d.show(errLine+"Напиши дату (ДД.ММ.ГГ) или выбери из списка", kb)
	}, func(s string) error {
		if _, err := resolveDay(s); err != nil {
			return fmt.Errorf("формат ДД.ММ.ГГ, например 07.09.26")
		}
		return nil
	})
}

func validRoom(s string) error {
	if n, err := strconv.Atoi(s); err != nil || n < 0 || n > 999 {
		return fmt.Errorf("кабинет — число от 0 до 999")
	}
	return nil
}

func validSubject(s string) error {
	if len([]rune(s)) > 40 {
		return fmt.Errorf("название длиннее 40 символов")
	}
	return nil
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
		tu.Entity("😇").CustomEmoji("5818920837645867167"),
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
		scheduleDel(ctx, query, parts)
	case "edit":
		scheduleEdit(ctx, query, parts[1:])
	case "set":
		scheduleSet(ctx, query, parts[1:])
	}
}
func scheduleDel(ctx *th.Context, query telego.CallbackQuery, p []string) {
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
			`<tg-emoji emoji-id="5818920837645867167">😇</tg-emoji><b> Напиши дату дня, которого хочешь удалить или выбери из списка</b>`).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(tu.InlineKeyboard(
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("Вчерашний").WithCallbackData("schedule:del:yesterday").WithStyle("primary"),
					tu.InlineKeyboardButton("Сегодняшний").WithCallbackData("schedule:del:today").WithStyle("danger"),
					tu.InlineKeyboardButton("Завтрашний").WithCallbackData("schedule:del:tomorrow").WithStyle("primary"),
				),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад к панели").WithCallbackData("admin").WithStyle("success")),
			)))
	if len(p) > 1 && p[1] != "" {
		switch p[1] {
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

}

var setGroups = map[string]Group{"a": Empty, "i": InfTec, "s": SocEco}

func groupCode(g Group) string {
	switch g {
	case InfTec:
		return "i"
	case SocEco:
		return "s"
	}
	return "a"
}

func scheduleSet(ctx *th.Context, query telego.CallbackQuery, p []string) {
	d := newAdminDlg(ctx, query, "schedule")

	if len(p) == 0 || p[0] == "" {
		txt := d.askDate("schedule:set")
		if txt == "" {
			return
		}
		p = []string{txt}
	}
	date, err := resolveDay(p[0])
	if err != nil {
		d.show("Не разобрал дату. Формат: ДД.ММ.ГГ")
		return
	}
	ds := date.Format(dateLayout)

	if len(p) == 1 {
		if day, ok := week.GetDay(date); ok && len(day.Entries) > 0 {
			d.show(fmt.Sprintf("На %s уже есть расписание (строк: %d). Что сделать?", formatHeaderDate(date), len(day.Entries)),
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("Редактировать").WithCallbackData("schedule:edit:"+ds).WithStyle("primary"),
					tu.InlineKeyboardButton("Заменить").WithCallbackData("schedule:set:"+ds+":1:a:cl").WithStyle("danger"),
				))
			return
		}
		p = []string{p[0], "1"}
	}

	n, err := strconv.Atoi(p[1])
	last := len(scheduleFor(date))
	if err != nil || n < 1 || n > last {
		d.show("Некорректный номер урока")
		return
	}
	code, act := "a", ""
	if len(p) > 2 {
		code = p[2]
	}
	if len(p) > 3 {
		act = p[3]
	}
	g, ok := setGroups[code]
	if !ok {
		return
	}

	finish := func() {
		day, ok := week.GetDay(date)
		if !ok || len(day.Entries) == 0 {
			d.show("Ничего не добавлено, расписание не создано")
			return
		}
		d.show(fmt.Sprintf("Готово: расписание на %s сохранено (строк: %d)", formatHeaderDate(date), len(day.Entries)),
			tu.InlineKeyboardRow(tu.InlineKeyboardButton("Проверить / изменить").
				WithCallbackData("schedule:edit:"+ds).WithStyle("primary")))
	}
	next := func() {
		if n >= last {
			finish()
			return
		}
		scheduleSet(ctx, query, []string{ds, strconv.Itoa(n + 1)})
	}

	switch act {
	case "ok":
		finish()
		return
	case "nx":
		next()
		return
	case "cl":
		week.DeleteDay(date)
		act = ""
	case "rm":
		if !hasLesson(date, n, g) {
			act = ""
		}
	}

	base := fmt.Sprintf("schedule:set:%s:%d:%s", ds, n, code)
	who := ""
	if g != Empty {
		who = " (" + entryLabel(LessonEntry{Group: g}) + ")"
	}

	if act != "rm" {
		sug, hasSug := prevWeekLesson(date, n, g)

		subject, room := "", 0
		if act == "nm" {
			if !hasSug {
				d.show("Предложенного названия нет, начни урок заново")
				return
			}
			subject, room = sug.Subject, sug.Room
		} else {
			var rows [][]telego.InlineKeyboardButton
			if hasSug {
				label := strings.TrimSpace(sug.Subject)
				if label == "" {
					label = "—"
				}
				rows = append(rows, tu.InlineKeyboardRow(
					tu.InlineKeyboardButton(label).WithCallbackData(base+":nm").WithStyle("primary")))
			}
			pick := func(label string, gr Group) telego.InlineKeyboardButton {
				to, style := groupCode(gr), "primary"
				if g == gr {
					to, style = "a", "success"
				}
				return tu.InlineKeyboardButton(label).
					WithCallbackData(fmt.Sprintf("schedule:set:%s:%d:%s", ds, n, to)).WithStyle(style)
			}
			rows = append(rows,
				tu.InlineKeyboardRow(pick("ИТ", InfTec), pick("СЭ", SocEco)),
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("Пропустить").WithCallbackData(base+":nx").WithStyle("danger"),
					tu.InlineKeyboardButton("Закончить").WithCallbackData(base+":ok"),
				))
			subject = d.askUntil(func(errLine string) {
				d.show(errLine+fmt.Sprintf("%s, урок %d%s. Напиши название", formatHeaderDate(date), n, who), rows...)
			}, validSubject)
			if subject == "" {
				return
			}
			if hasSug {
				room = sug.Room
			}
		}
		idx, err := week.UpsertEntry(date, LessonEntry{Number: n, Subject: subject, Room: room, Group: g})
		if err != nil {
			d.show(html.EscapeString(err.Error()))
			return
		}

		var rows [][]telego.InlineKeyboardButton
		if hasSug {
			rows = append(rows, tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("каб. "+strconv.Itoa(room)).WithCallbackData(base+":rm").WithStyle("primary")))
		}
		txt := d.askUntil(func(errLine string) {
			d.show(errLine+fmt.Sprintf("Урок %d%s. Напиши кабинет", n, who), rows...)
		}, validRoom)
		if txt == "" {
			return
		}
		r, _ := strconv.Atoi(txt)
		if err := week.EditEntry(date, idx, func(le *LessonEntry) { le.Room = r }); err != nil {
			d.show(html.EscapeString(err.Error()))
			return
		}
	}

	if g == Empty {
		next()
		return
	}
	other := SocEco
	if g == SocEco {
		other = InfTec
	}
	var rows [][]telego.InlineKeyboardButton
	if !hasLesson(date, n, other) {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Тот же урок для "+entryLabel(LessonEntry{Group: other})).
				WithCallbackData(fmt.Sprintf("schedule:set:%s:%d:%s", ds, n, groupCode(other))).WithStyle("primary")))
	}
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Дальше").WithCallbackData(base+":nx").WithStyle("success")))
	d.show(fmt.Sprintf("Урок %d%s сохранён", n, who), rows...)
}
func scheduleEdit(ctx *th.Context, query telego.CallbackQuery, p []string) {
	d := newAdminDlg(ctx, query, "schedule")
	show := d.show
	if len(p) == 0 || p[0] == "" {
		txt := d.askDate("schedule:edit")
		if txt == "" {
			return
		}
		p = []string{txt}
	}
	date, err := resolveDay(p[0])
	if err != nil {
		show("Не разобрал дату. Формат: ДД.ММ.ГГ")
		return
	}
	day, ok := week.GetDay(date)
	if !ok {
		show("На эту дату расписания нет")
		return
	}
	ds := date.Format(dateLayout)
	if len(p) == 1 {
		var nums []int
		for _, e := range day.Entries {
			if !slices.Contains(nums, e.Number) {
				nums = append(nums, e.Number)
			}
		}
		slices.Sort(nums)
		var btns []telego.InlineKeyboardButton
		for _, n := range nums {
			btns = append(btns, tu.InlineKeyboardButton(strconv.Itoa(n)).
				WithCallbackData(fmt.Sprintf("schedule:edit:%s:%d", ds, n)).WithStyle("primary"))
		}
		var rows [][]telego.InlineKeyboardButton
		for c := range slices.Chunk(btns, 4) {
			rows = append(rows, tu.InlineKeyboardRow(c...))
		}
		show("Какой урок изменить? ("+formatHeaderDate(date)+")", rows...)
		return
	}
	n, err := strconv.Atoi(p[1])
	if err != nil {
		show("Некорректный номер урока")
		return
	}
	if len(p) == 2 {
		var idxs []int
		for i, e := range day.Entries {
			if e.Number == n {
				idxs = append(idxs, i)
			}
		}
		switch {
		case len(idxs) == 0:
			show("Такого урока нет")
			return
		case len(idxs) > 1:
			var rows [][]telego.InlineKeyboardButton
			for _, i := range idxs {
				e := day.Entries[i]
				rows = append(rows, tu.InlineKeyboardRow(
					tu.InlineKeyboardButton(entryLabel(e)+" · "+strings.TrimSpace(e.Subject)).
						WithCallbackData(fmt.Sprintf("schedule:edit:%s:%d:%d", ds, n, i)).WithStyle("primary")))
			}
			show(fmt.Sprintf("Урок %d: у какой группы менять?", n), rows...)
			return
		}
		p = append(p, strconv.Itoa(idxs[0]))
	}
	idx, err := strconv.Atoi(p[2])
	if err != nil || idx < 0 || idx >= len(day.Entries) || day.Entries[idx].Number != n {
		show("Расписание успело измениться, начни заново")
		return
	}
	e := day.Entries[idx]
	base := fmt.Sprintf("schedule:edit:%s:%d:%d", ds, n, idx)
	who := ""
	if e.Group != Empty || e.Addi {
		who = " (" + entryLabel(e) + ")"
	}
	if len(p) == 3 {
		label := strings.TrimSpace(e.Subject)
		if label == "" {
			label = "—"
		}
		name := d.askUntil(func(errLine string) {
			show(errLine+fmt.Sprintf("Урок %d%s. Напиши новое название или нажми на прежнее", n, who),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton(label).WithCallbackData(base+":nm").WithStyle("primary")))
		}, validSubject)
		if name == "" {
			return
		}
		if err := week.EditEntry(date, idx, func(le *LessonEntry) { le.Subject = name }); err != nil {
			show(html.EscapeString(err.Error()))
			return
		}
		p = append(p, "nm")
	}
	if p[3] == "nm" {
		txt := d.askUntil(func(errLine string) {
			show(errLine+fmt.Sprintf("Урок %d%s. Напиши новый кабинет или нажми на прежний", n, who),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("каб. "+strconv.Itoa(e.Room)).WithCallbackData(base+":rm").WithStyle("primary")))
		}, validRoom)
		if txt == "" {
			return
		}
		room, _ := strconv.Atoi(txt)
		if err := week.EditEntry(date, idx, func(le *LessonEntry) { le.Room = room }); err != nil {
			show(html.EscapeString(err.Error()))
			return
		}
	}
	if day, ok = week.GetDay(date); !ok || idx >= len(day.Entries) {
		return
	}
	e = day.Entries[idx]
	show(fmt.Sprintf("Готово: урок %d%s · %s, каб. %d", n, who, html.EscapeString(strings.TrimSpace(e.Subject)), e.Room),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Другой урок").WithCallbackData("schedule:edit:"+ds).WithStyle("primary")))
}

func resolveDay(s string) (time.Time, error) {
	y, m, dd := time.Now().Date()
	today := time.Date(y, m, dd, 0, 0, 0, 0, time.UTC)
	switch s {
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	case "today":
		return today, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	}
	return time.Parse(dateLayout, s)
}

func prevWeekLesson(date time.Time, n int, g Group) (LessonEntry, bool) {
	for k := 1; k <= 8; k++ {
		ref, ok := week.GetDay(date.AddDate(0, 0, -7*k))
		if !ok || len(ref.Entries) == 0 {
			continue
		}
		var cand []LessonEntry
		for _, e := range ref.Entries {
			if e.Number == n && !e.Addi {
				cand = append(cand, e)
			}
		}
		if len(cand) == 0 {
			return LessonEntry{}, false
		}
		if i := slices.IndexFunc(cand, func(e LessonEntry) bool { return e.Group == g }); i >= 0 {
			return cand[i], true
		}
		return cand[0], true
	}
	return LessonEntry{}, false
}

func hasLesson(date time.Time, n int, g Group) bool {
	day, ok := week.GetDay(date)
	return ok && slices.ContainsFunc(day.Entries, func(e LessonEntry) bool {
		return e.Number == n && e.Group == g && !e.Addi
	})
}

func entryLabel(e LessonEntry) string {
	switch {
	case e.Group == InfTec:
		return "И-Т"
	case e.Group == SocEco:
		return "С-Э"
	case e.Addi:
		return "Доп."
	}
	return "Общий"
}
func announcementCallback(ctx *th.Context, query telego.CallbackQuery, s string) {
	if !isAdmin(query.From.ID) {
		ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Нет доступа"))
		return
	}
	if s == "" {
		adminpanel(query, ctx, 2)
		return
	}
	parts := strings.Split(strings.TrimPrefix(s, ":"), ":")
	switch parts[0] {
	case "make":
		announcementMake(ctx, query, parts[1:])
	case "drafts":
		announcementList(ctx, query, "Черновики")
	case "publish":
		if len(parts) == 1 {
			announcementList(ctx, query, "Какое оповещение опубликовать?")
			return
		}
		announcementItem(ctx, query, parts[1:])
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
		const emoji = `<tg-emoji emoji-id="5818920837645867167">😇</tg-emoji><b> `

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
		chatID, msgID := query.Message.GetChat().ChatID(), query.Message.GetMessageID()
		const emoji = `<tg-emoji emoji-id="5818920837645867167">😇</tg-emoji><b> `
		toPanel := tu.InlineKeyboard(tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Назад").WithCallbackData("members").WithStyle("success")))
		fail := func(text string) {
			ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID, text).WithReplyMarkup(toPanel))
		}

		ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID,
			emoji+"Отправь ID пользователя</b>\nОн должен хотя бы раз нажать /start у бота",
		).WithParseMode("HTML").WithReplyMarkup(toPanel))
		wctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		upd, err := waiter.WaitForMessage(wctx, chatID)
		if err != nil || upd.Message == nil {
			return
		}
		ctx.Bot().DeleteMessage(ctx, tu.Delete(chatID, upd.Message.MessageID))
		id, perr := strconv.ParseInt(strings.TrimSpace(upd.Message.Text), 10, 64)
		if perr != nil || id <= 0 {
			fail("ID должен быть положительным числом")
			return
		}
		if _, exists := Groups.Get(id); exists {
			memberCard(ctx, chatID, msgID, id, false)
			return
		}
		chat, cerr := ctx.Bot().GetChat(ctx, &telego.GetChatParams{ChatID: tu.ID(id)})
		if cerr != nil {
			fail("Бот не знает такого пользователя: пусть сначала нажмёт /start")
			return
		}
		Groups.Update(id, func(c *Classmate) {
			if chat.Username != "" {
				c.TelegName = "@" + chat.Username
			}
			if n, err := cleanName(chat.FirstName); err == nil {
				c.Name = n
			}
			c.ManualSettings = true
		})
		memberCard(ctx, chatID, msgID, id, false)
	case "edit":
		chatID, msgID := query.Message.GetChat().ChatID(), query.Message.GetMessageID()
		const emoji = `<tg-emoji emoji-id="5199597179311788893">😇</tg-emoji><b> `

		var (
			id    int64
			found bool
		)
		hasID := len(parts) > 1 && parts[1] != ""
		if hasID {
			if v, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				id = v
				_, found = Groups.Get(id)
			}
		}
		action, arg := "", ""
		if len(parts) > 2 {
			action = parts[2]
		}
		if len(parts) > 3 {
			arg = parts[3]
		}
		annMenu := false
		card := func() { memberCard(ctx, chatID, msgID, id, annMenu) }
		switch {
		case !hasID:
			var baty []telego.InlineKeyboardButton
			for _, uid := range Groups.FirstIDs(4) {
				if u, ok := Groups.Get(uid); ok {
					baty = append(baty, tu.InlineKeyboardButton(displayName(uid, u)).
						WithCallbackData(fmt.Sprintf("members:edit:%d", uid)).
						WithStyle("primary"))
				}
			}
			ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID,
				emoji+`Выбери человека для редактирования</b>`,
			).WithParseMode("HTML").WithReplyMarkup(tu.InlineKeyboard(
				tu.InlineKeyboardRow(baty...),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад к панели").WithCallbackData("members").WithStyle("success")),
			)))

		case !found:
			return

		case action == "name":
			ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID,
				emoji+`Напиши новое имя</b>`,
			).WithParseMode("HTML"))
			upd, err := waiter.WaitForMessage(ctx, chatID)
			if err != nil || upd.Message == nil {
				return
			}
			name, nerr := cleanName(upd.Message.Text)
			if nerr != nil {
				ctx.Bot().SendMessage(ctx, tu.Message(chatID, "Не подходит: "+nerr.Error()))
			} else {
				Groups.Update(id, func(c *Classmate) { c.Name = name; c.ManualSettings = true })
			}
			card()
		case action == "adm":
			Groups.Update(id, func(c *Classmate) { c.Admin = !c.Admin })
			card()
		case action == "grp":
			g := map[string]Group{"it": InfTec, "se": SocEco, "no": Empty}[arg]
			Groups.SetGroup(id, g)
			Groups.Update(id, func(c *Classmate) { c.ManualSettings = true })
			card()
		case action == "bd":
			ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID,
				emoji+`Напиши новый день рождения</b>`,
			).WithParseMode("HTML"))
			upd, err := waiter.WaitForMessage(ctx, chatID)
			if err != nil || upd.Message == nil {
				return
			}
			bd, nerr := time.Parse(dateLayout, upd.Message.Text)
			if nerr != nil {
				ctx.Bot().SendMessage(ctx, tu.Message(chatID, "Не подходит: "+nerr.Error()))
			} else {
				Groups.Update(id, func(c *Classmate) {
					c.Birthday = Birthday{Day: bd.Day(), Month: int(bd.Month()), Year: bd.Year()}
					c.ManualSettings = true
				})
			}
			card()
		case action == "announcement":
			switch arg {
			case "self", "other":
				Groups.Update(id, func(c *Classmate) {
					own := c.Announcement == "all" || c.Announcement == "self"
					other := c.Announcement == "all" || c.Announcement == "other"
					if arg == "self" {
						own = !own
					} else {
						other = !other
					}
					switch {
					case own && other:
						c.Announcement = "all"
					case own:
						c.Announcement = "self"
					case other:
						c.Announcement = "other"
					default:
						c.Announcement = "non"
					}
				})
			case "inv":
				Groups.Update(id, func(c *Classmate) { c.Innovations = !c.Innovations })
			case "duty":
				Groups.Update(id, func(c *Classmate) { c.Duty = !c.Duty })
			}
			Groups.Update(id, func(c *Classmate) {
				c.ManualSettings = true
				c.Notify = c.Announcement == "all" && c.Duty && c.Innovations
			})
			annMenu = true
			card()
		// case action == "":
		default:
			card()
		}
		return
	}
}
func memberCard(ctx *th.Context, chatID telego.ChatID, msgID int, id int64, annMenu bool) {
	dood, _ := Groups.Get(id)
	pfx := fmt.Sprintf("members:edit:%d:", id)
	back := tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData("members:edit").WithStyle("success"))

	dood, _ = Groups.Get(id)
	bd := "не указан"
	if dood.Birthday.IsSet() {
		bd = fmt.Sprintf("%02d.%02d", dood.Birthday.Day, dood.Birthday.Month)
		if dood.Birthday.Year != 0 {
			bd += fmt.Sprintf(".%d", dood.Birthday.Year)
		}
	}
	ownBd := dood.Announcement == "all" || dood.Announcement == "self"
	otherBd := dood.Announcement == "all" || dood.Announcement == "other"
	notices := []struct {
		name string
		on   bool
	}{
		{"Свой день рождения", ownBd},
		{"Дни рождения одноклассников", otherBd},
		{"Нововведения бота", dood.Innovations},
		{"График дежурств", dood.Duty},
	}
	parts := []tu.MessageEntityCollection{
		tu.Entity("🍎").CustomEmoji("5818920837645867167"), tu.Entity(" "), tu.Entity(displayName(id, dood)).Bold(), tu.Entity("\n"),
		tu.Entity("🏷").CustomEmoji("5987802868734760945"), tu.Entity(" Группа: ").Bold(), tu.Entity(dood.Group.RuString()), tu.Entity("\n"),
		tu.Entity("🎂").CustomEmoji("5994502837327892086"), tu.Entity(" День рождения: ").Bold(), tu.Entity(bd), tu.Entity("\n"),
		tu.Entity("🔔").CustomEmoji("5909201569898827582"), tu.Entity(" Уведомления:").Bold(),
	}
	for i, n := range notices {
		branch := " ┣"
		if i == len(notices)-1 {
			branch = " ┗"
		}
		icon, eid := "❌", "5778527486270770928"
		if n.on {
			icon, eid = "✅", "5776375003280838798"
		}
		parts = append(parts,
			tu.Entity("\n"+branch+" "+n.name+" "),
			tu.Entity(icon).CustomEmoji(eid),
		)
	}
	parts = append(parts, tu.Entity("\n"),
		tu.Entity("😇").CustomEmoji("5879841310902324730"), tu.Entity(" Выберите, что изменить: ").Bold())
	txt, entities := tu.MessageEntities(parts...)
	kb := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Имя").WithCallbackData(pfx+"name").WithStyle("primary"),
			tu.InlineKeyboardButton("Д.Р.").WithCallbackData(pfx+"bd").WithStyle("primary"),
			tu.InlineKeyboardButton("Уведомления").WithCallbackData(pfx+"announcement").WithStyle("primary"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Без группы").WithCallbackData(pfx+"grp:no").WithStyle("danger"),
			tu.InlineKeyboardButton("СЭ").WithCallbackData(pfx+"grp:se").WithStyle("success"),
			tu.InlineKeyboardButton("ИТ").WithCallbackData(pfx+"grp:it").WithStyle("success"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Админ").WithCallbackData(pfx+"adm").WithStyle("danger")),
		back,
	)
	if annMenu {
		kb = tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Свой Д.Р.").WithCallbackData(pfx+"announcement:self").WithStyle("primary"),
				tu.InlineKeyboardButton("Д.Р. класса").WithCallbackData(pfx+"announcement:other").WithStyle("primary"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Нововведения").WithCallbackData(pfx+"announcement:inv").WithStyle("primary"),
				tu.InlineKeyboardButton("Дежурства").WithCallbackData(pfx+"announcement:duty").WithStyle("primary"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Назад").WithCallbackData(fmt.Sprintf("members:edit:%d", id)).WithStyle("success"),
			),
		)
	}
	ctx.Bot().EditMessageText(ctx, tu.EditMessageText(chatID, msgID, txt).
		WithEntities(entities...).
		WithReplyMarkup(kb))

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

	for range 3 {
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
