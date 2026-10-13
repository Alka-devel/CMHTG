package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type job struct {
	ctx context.Context
	bot *telego.Bot
	i   int64
}

func startScheduler(ctx context.Context, bot *telego.Bot, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		runCheck(ctx, bot)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				runCheck(ctx, bot)
			}
		}
	}()
}

func runCheck(ctx context.Context, bot *telego.Bot) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("scheduler panic:", r)
		}
	}()
	lstCls := Groups.IDs()
	for _, i := range lstCls {
		j := job{ctx, bot, i}
		j.lastRememberCheck()
		j.selfBirthdayCheck()
		j.otherBithdayCheck(lstCls)
	}
}
func (j *job) otherBithdayCheck(l []int64) {
	return
	var birth_id int64
	_ = birth_id
	for _, i := range l {
		_ = i
	}
}
func (j *job) selfBirthdayCheck() (int64, bool) {
	if dood, ok := Groups.Get(j.i); ok && (dood.Announcement == "all" || dood.Announcement == "self") && dood.Birthday.IsToday() && time.Now().Hour() > 10 && dood.LastRemember.Format("2006-01-02") != time.Now().Format("2006-01-02") {
		_, er := j.bot.SendMessage(j.ctx, tu.MessageWithEntities(
			tu.ID(j.i),
			tu.Entity("😇").CustomEmoji("5906995262378741881"),
			tu.Entityf(" Привет, %s, тебе сегодня исполнилось %d лет!\nХоть я и бездушная машина, но всё равно хочу, чтобы у тебя в жизни было много приключений, счастья и смешных моментов!",
				dood.Name, time.Now().Year()-dood.Birthday.Year).Bold(),
			tu.Entity("\nС днём рождения!").Bold(),
			tu.Entity("😇").CustomEmoji("5465263910414195580"),
		))
		if er != nil {
			fmt.Println("birth err: ", er)
		}
		return j.i, true
	}
	return j.i, false
}
func (j *job) lastRememberCheck() {
	if dood, ok := Groups.Get(j.i); ok && !dood.Configured && time.Since(dood.LastRemember) > 6*time.Hour {
		_, er := j.bot.SendMessage(j.ctx, tu.MessageWithEntities(
			tu.ID(j.i),
			tu.Entity("😇").CustomEmoji("5875465628285931233"),
			tu.Entity("Привет! У тебя не настроена главная информация!\n\n"),
			tu.Entity("😇").CustomEmoji("5879770735999717115"),
			tu.Entity("Отправь команду /start и я помогу тебе настроить личный кабинет!"),
		))
		if er != nil {
			return
		}
		Groups.Update(j.i, func(u *Classmate) {
			u.LastRemember = time.Now()
			// us, usErr := bot.GetChat(ctx, &telego.GetChatParams{
			// 	ChatID: tu.ID(i),
			// })
			// if usErr != nil || us.Type != "private" ||us.IsForum {
			// 	return
			// }
		})
	}
}
