package main

import (
	"context"
	"log"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

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
		if dood, ok := Groups.Get(i); ok && !dood.Configured && time.Since(dood.LastRemember) > 6*time.Hour {
			_, _ = bot.SendMessage(ctx, tu.MessageWithEntities(
				tu.ID(i),
				tu.Entity("😇").CustomEmoji("5875465628285931233"),
				tu.Entity("Привет! У тебя не настроена главная информация!\n\n"),
				tu.Entity("😇").CustomEmoji("5879770735999717115"),
				tu.Entity("Отправь команду /start и я помогу тебе настроить личный кабинет!"),
			))
			Groups.Update(i, func(u *Classmate) {
				u.LastRemember = time.Now()
				us, usErr := bot.GetChat(ctx, &telego.GetChatParams{
					ChatID: tu.ID(i),
				})
				if usErr != nil || us.Type != "private" ||us.IsForum {
					return
				}
			})
		}
	}
}
