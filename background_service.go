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
		fixNames()
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
	ids := Groups.IDs()
	for _, id := range ids {
		if id < 0 {
			continue
		}
		runJob(ctx, bot, id, ids)
	}
}

func runJob(ctx context.Context, bot *telego.Bot, id int64, ids []int64) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("scheduler panic:", id, r)
		}
	}()
	j := job{ctx, bot, id}
	j.adminCheck()
	j.mainSpecCheck()
	j.lastRememberCheck()
	j.selfBirthdayCheck()
	j.otherBithdayCheck(ids)
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func yearsWord(n int) string {
	if n%100 >= 11 && n%100 <= 14 {
		return "лет"
	}
	switch n % 10 {
	case 1:
		return "год"
	case 2, 3, 4:
		return "года"
	}
	return "лет"
}

func whoLabel(c Classmate) string {
	name := c.Name
	if name == "" {
		name = "без имени"
	}
	if c.TelegName != "" {
		return name + ", " + c.TelegName
	}
	return name
}

func (j *job) mainSpecCheck() {
	dood, ok := Groups.Get(j.i)
	if !ok {
		return
	}
	hch, err := j.bot.GetChat(j.ctx, &telego.GetChatParams{ChatID: tu.ID(j.i)})
	if err != nil {
		return
	}
	uname := ""
	if hch.Username != "" {
		uname = "@" + hch.Username
	}
	if uname == dood.TelegName {
		return
	}
	Groups.Update(j.i, func(c *Classmate) { c.TelegName = uname })
}

func (j *job) otherBithdayCheck(ids []int64) {
	me, ok := Groups.Get(j.i)
	now := time.Now()
	if !ok || !(j.i == ownerID || me.Announcement == "all" || me.Announcement == "other") ||
		now.Hour() <= 10 || sameDay(Groups.LastSent(j.i, NoticeOtherBD), now) {
		return
	}
	sent := 0
	for _, id := range ids {
		if id == j.i {
			continue
		}
		ch, ok := Groups.Get(id)
		if !ok || (ch.AnonBdMode && j.i != ownerID) || !ch.Birthday.IsIn(1) {
			continue
		}
		_, err := j.bot.SendMessage(j.ctx, tu.MessageWithEntities(tu.ID(j.i),
			tu.Entity("😇").CustomEmoji("5906995262378741881"),
			tu.Entityf(" У твоего одноклассника (%s) завтра день рождения, не забудь поздравить его! ", whoLabel(ch)).Bold(),
			tu.Entity("😇").CustomEmoji("5339255565723903023"),
		))
		if err != nil {
			log.Println("other birthday:", err)
			continue
		}
		sent++
	}
	if sent > 0 {
		Groups.MarkSent(j.i, NoticeOtherBD, now)
	}
}

func (j *job) selfBirthdayCheck() {
	dood, ok := Groups.Get(j.i)
	now := time.Now()
	if !ok || !(j.i == ownerID || dood.Announcement == "all" || dood.Announcement == "self") ||
		!dood.Birthday.IsToday() || now.Hour() <= 10 ||
		sameDay(Groups.LastSent(j.i, NoticeSelfBD), now) {
		return
	}
	age := ""
	if dood.Birthday.Year != 0 {
		n := now.Year() - dood.Birthday.Year
		age = fmt.Sprintf(", тебе сегодня исполнилось %d %s", n, yearsWord(n))
	}
	_, err := j.bot.SendMessage(j.ctx, tu.MessageWithEntities(
		tu.ID(j.i),
		tu.Entity("😇").CustomEmoji("5906995262378741881"),
		tu.Entityf(" Привет, %s%s!\nХоть я и бездушная машина, но всё равно хочу, чтобы у тебя в жизни было много приключений, счастья и смешных моментов!",
			dood.Name, age).Bold(),
		tu.Entity("\nС днём рождения!").Bold(),
		tu.Entity("😇").CustomEmoji("5465263910414195580"),
	))
	if err != nil {
		log.Println("birth err:", err)
		return
	}
	Groups.MarkSent(j.i, NoticeSelfBD, now)
}
func (j *job) adminCheck() {
	if dood, ok := Groups.Get(j.i); ok && dood.Admin {
		Groups.Update(j.i, func(c *Classmate) {
			c.Admin = true
		})
	}
}
func (j *job) lastRememberCheck() {
	if dood, ok := Groups.Get(j.i); ok && !dood.Configured &&
		time.Since(Groups.LastSent(j.i, NoticeSetup)) > 6*time.Hour {
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
		Groups.MarkSent(j.i, NoticeSetup, time.Now())
	}
}
