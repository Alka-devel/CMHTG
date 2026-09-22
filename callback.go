package main

import (
	"bytes"
	"fmt"
	"log"
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
func callbackHan(bh *th.BotHandler) {
	bh.HandleCallbackQuery(func(ctx *th.Context, query telego.CallbackQuery) error {
		chID := query.Message.GetChat().ChatID()
		if strings.HasPrefix(query.Data, "showScheduleImg") {
			_ = ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{})
			if dsa := strings.Split(strings.TrimPrefix(query.Data, "showScheduleImg:"), ":"); dsa[0] != "" {
				force := false
				if dsa[1] == "t" {
					force = true
				}
				day, ok := week.GetDayByDate(dsa[0])
				deleteQueryMessage(ctx, query)
				if !ok {
					_, _ = ctx.Bot().SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(),
						fmt.Sprintf("Расписание на %s не найдено.", dsa[0])))
					return nil
				}
				schImg(ctx, query.Message.GetChat().ChatID(), day, force)
				return nil
			}
		}
		_ = ctx.Bot().AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{})
		switch query.Data {
		case "showSchedule":
			deleteQueryMessage(ctx, query)
			day, ok := week.GetDay(time.Now())
			if !ok {
				_, err := ctx.Bot().SendMessage(ctx, tu.Message(chID, "Расписание на сегодня не найдено."))
				return err
			}
			schImg(ctx, chID, day, false)
		case "nothing":
			deleteQueryMessage(ctx, query)
		case "it":
			Groups.SetGroup(chID.ID, InfTec)
		case "se":
			Groups.SetGroup(chID.ID, SocEco)
		}
		return nil
	}, th.AnyCallbackQueryWithMessage())
}

func schImg(ctx *th.Context, chID telego.ChatID, d ScheduleDay, force bool) {
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
	img, err := RenderScheduleImage(d, currentIndex, fmtDur(st.TimeLeft), DefaultScale)
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
