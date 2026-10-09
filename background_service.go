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
			log.Println("scheduler panic:", r) // паника в горутине иначе уронит весь бот
		}
	}()
	lstCls := Groups.IDs()
	for _, i := range lstCls {
		if dood, ok := Groups.Get(i); ok && !dood.Configured && time.Since(dood.LastRemember) > 6*time.Hour {
			bot.SendMessage(ctx, tu.MessageWithEntities(
				tu.ID(i),
				tu.Entity("😇").CustomEmoji("5875465628285931233"),
				tu.Entity("Привет! У тебя не настроена главная информация!\n"),
				tu.Entity("\n"),
				tu.Entity("😇").CustomEmoji("5879770735999717115"),
				tu.Entity("Отправь команду /start и я помогу тебе настроить личный кабинет!"),
			))
			dood.LastRemember = time.Now()
		}
	}
}
