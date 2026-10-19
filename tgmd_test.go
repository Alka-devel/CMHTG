package main

import (
	"testing"
	"time"

	"github.com/mymmrac/telego"
)

func TestEntitiesToMarkdownV2(t *testing.T) {
	ent := func(typ string, off, n int) telego.MessageEntity {
		return telego.MessageEntity{Type: typ, Offset: off, Length: n}
	}
	cases := []struct {
		name string
		text string
		ents []telego.MessageEntity
		want string
	}{
		{"экранирование", "a.b_c", nil, `a\.b\_c`},
		{"жирный", "Привет, мир", []telego.MessageEntity{ent(telego.EntityTypeBold, 0, 6)}, "*Привет*, мир"},
		{"кастомное эмодзи", "😇 ok", []telego.MessageEntity{
			{Type: telego.EntityTypeCustomEmoji, Offset: 0, Length: 2, CustomEmojiID: "123"},
		}, "![😇](tg://emoji?id=123) ok"},
		{"пересечение", "abcde", []telego.MessageEntity{
			ent(telego.EntityTypeBold, 0, 3), ent(telego.EntityTypeItalic, 2, 3),
		}, "*ab_c_*_de_"},
		{"ссылка", "см. тут", []telego.MessageEntity{
			{Type: telego.EntityTypeTextLink, Offset: 4, Length: 3, URL: "https://a.b/(x)"},
		}, `см\. [тут](https://a.b/(x\))`},
		{"цитата", "a\nb", []telego.MessageEntity{ent(telego.EntityTypeBlockquote, 0, 3)}, ">a\n>b"},
		{"код", "x_y", []telego.MessageEntity{ent(telego.EntityTypeCode, 0, 3)}, "`x_y`"},
	}
	for _, c := range cases {
		if got := entitiesToMarkdownV2(c.text, c.ents); got != c.want {
			t.Errorf("%s: получили %q, ждали %q", c.name, got, c.want)
		}
	}
}

func TestAnnouncementRoundTrip(t *testing.T) {
	a := announcement{Type: "announcement", Words: 5, Title: `*Привет ![😇](tg://emoji?id=1)*`, Body: "Текст\n\nвторой абзац\\."}
	a.Created, _ = time.Parse(time.RFC3339, "2026-10-20T14:35:10+10:00")
	got, err := parseAnnouncement(a.markdown())
	if err != nil {
		t.Fatal(err)
	}
	if got != a {
		t.Errorf("получили %+v, ждали %+v", got, a)
	}
}
