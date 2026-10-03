package main

// gdz_send.go — отправка GDZResult в Telegram и команда /gdz.

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	maxTextLen    = 3500 // лимит Telegram — 4096; с запасом на теги
	maxAlbum      = 10   // максимум элементов в sendMediaGroup
	maxCaptionLen = 1000 // лимит подписи — 1024
)

// ───────────────────────────── КОМАНДА /gdz ─────────────────────────────

// gdzCom заменяет rhwCom: /gdz <предмет> <номер>. Старый формат «/gdz 74» = алгебра.
func gdzCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chID := update.Message.Chat.ChatID()
		reply := func(text string) {
			_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, text))
		}

		fields := strings.Fields(update.Message.Text)
		if len(fields) > 0 {
			fields = fields[1:] // первое слово — сама команда (/gdz или /gdz@bot)
		}
		subj, arg := resolveGDZ(fields)
		if subj == nil {
			reply(gdzUsage())
			return nil
		}

		_ = ctx.Bot().SendChatAction(ctx, tu.ChatAction(chID, telego.ChatActionUploadPhoto))
		res, err := FetchGDZ(ctx, subj, arg)
		if err != nil {
			reply("Не получилось: " + err.Error())
			return nil
		}
		if err := sendGDZ(ctx, chID, res); err != nil {
			log.Println("gdz: отправка:", err)
			reply("Не всё удалось отправить: " + err.Error())
		}
		return nil
	}, th.CommandEqual("gdz"))
}

func resolveGDZ(args []string) (*gdzSubject, string) {
	if len(args) == 0 {
		return nil, ""
	}
	if s, ok := findGDZSubject(args[0]); ok {
		return s, strings.Join(args[1:], " ")
	}
	if reNum.MatchString(args[0]) { // старый формат: /gdz 74
		s, _ := findGDZSubject("алг")
		return s, strings.Join(args, " ")
	}
	return nil, ""
}

func gdzUsage() string {
	var sb strings.Builder
	sb.WriteString("Формат: /gdz <предмет> <номер>\n\n")
	for _, s := range gdzSubjects {
		fmt.Fprintf(&sb, "/gdz %s — %s\n", s.Usage, s.Name)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// ───────────────────────────── ОТПРАВКА ─────────────────────────────

// sendGDZ отправляет кусочки по порядку: подряд идущие картинки — альбомами
// (по 10), текст — сообщениями ≤ maxTextLen. Заголовок страницы идёт подписью
// к первой картинке, а если ответ начинается с текста — жирной первой строкой.
func sendGDZ(ctx *th.Context, chID telego.ChatID, res GDZResult) error {
	caption, titleHTML := "", ""
	if title := strings.TrimSpace(res.Title); title != "" {
		if len(res.Parts) > 0 && res.Parts[0].Image != "" {
			caption = title
		} else {
			titleHTML = "<b>" + html.EscapeString(title) + "</b>\n\n"
		}
	}

	var imgs []string
	var failed int
	flushImages := func() {
		if len(imgs) == 0 {
			return
		}
		failed += sendImages(ctx, chID, imgs, res.URL, caption)
		caption = ""
		imgs = nil
	}

	for _, p := range res.Parts {
		if p.Image != "" {
			imgs = append(imgs, p.Image)
			continue
		}
		flushImages()
		for _, chunk := range splitText(titleHTML+p.Text, maxTextLen) {
			sendHTML(ctx, chID, chunk)
		}
		titleHTML = ""
	}
	flushImages()

	if failed > 0 {
		return fmt.Errorf("не удалось отправить картинок: %d", failed)
	}
	return nil
}

func sendHTML(ctx *th.Context, chID telego.ChatID, text string) {
	_, err := ctx.Bot().SendMessage(ctx, tu.Message(chID, text).WithParseMode(telego.ModeHTML))
	if err != nil {
		// битая разметка (например, длинную строку разрезали внутри тега) —
		// повторяем без форматирования
		log.Println("gdz: sendMessage(HTML):", err)
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, plainText(text)))
	}
	time.Sleep(300 * time.Millisecond)
}

type gdzFile struct {
	name string
	data []byte
}

// sendImages скачивает картинки (Telegram сам с reshak.ru не скачает — DDoS-Guard)
// и отправляет альбомами. Возвращает число картинок, которые не удалось доставить.
func sendImages(ctx *th.Context, chID telego.ChatID, urls []string, referer, caption string) (failed int) {
	var files []gdzFile
	for i, u := range urls {
		data, err := DownloadImage(ctx, u, referer)
		if err != nil {
			log.Println("gdz: скачивание:", u, err)
			failed++
			continue
		}
		files = append(files, gdzFile{name: fmt.Sprintf("gdz_%02d%s", i+1, imgExt(u)), data: data})
		time.Sleep(150 * time.Millisecond)
	}

	if caption = plainText(caption); utf8.RuneCountInString(caption) > maxCaptionLen {
		caption = string([]rune(caption)[:maxCaptionLen])
	}

	for start := 0; start < len(files); start += maxAlbum {
		chunk := files[start:min(start+maxAlbum, len(files))]
		c := ""
		if start == 0 {
			c = caption
		}
		failed += sendChunk(ctx, chID, chunk, c)
		time.Sleep(400 * time.Millisecond)
	}
	return failed
}

func sendChunk(ctx *th.Context, chID telego.ChatID, chunk []gdzFile, caption string) (failed int) {
	if len(chunk) == 1 {
		if err := sendOne(ctx, chID, chunk[0], caption); err != nil {
			return 1
		}
		return 0
	}
	media := make([]telego.InputMedia, 0, len(chunk))
	for i, f := range chunk {
		m := tu.MediaPhoto(tu.FileFromReader(bytes.NewReader(f.data), f.name))
		if i == 0 && caption != "" {
			m = m.WithCaption(caption)
		}
		media = append(media, m)
	}
	if _, err := ctx.Bot().SendMediaGroup(ctx, tu.MediaGroup(chID, media...)); err != nil {
		// одна неподходящая картинка роняет весь альбом — шлём по одной
		log.Println("gdz: sendMediaGroup:", err)
		for i, f := range chunk {
			c := ""
			if i == 0 {
				c = caption
			}
			if err := sendOne(ctx, chID, f, c); err != nil {
				failed++
			}
			time.Sleep(300 * time.Millisecond)
		}
	}
	return failed
}

// sendOne шлёт фото; если Telegram его не принял (слишком длинная/узкая и т.п.) —
// отправляет тот же файл документом.
func sendOne(ctx *th.Context, chID telego.ChatID, f gdzFile, caption string) error {
	p := tu.Photo(chID, tu.FileFromReader(bytes.NewReader(f.data), f.name))
	if caption != "" {
		p = p.WithCaption(caption)
	}
	if _, err := ctx.Bot().SendPhoto(ctx, p); err == nil {
		return nil
	} else {
		log.Println("gdz: sendPhoto:", f.name, err)
	}
	d := tu.Document(chID, tu.FileFromReader(bytes.NewReader(f.data), f.name))
	if caption != "" {
		d = d.WithCaption(caption)
	}
	if _, err := ctx.Bot().SendDocument(ctx, d); err != nil {
		log.Println("gdz: sendDocument:", f.name, err)
		return err
	}
	return nil
}

// ───────────────────────────── УТИЛИТЫ ─────────────────────────────

func imgExt(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		switch ext := strings.ToLower(path.Ext(u.Path)); ext {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif":
			return ext
		}
	}
	return ".png"
}

// plainText убирает теги и разэкранирует сущности.
func plainText(s string) string {
	return html.UnescapeString(tagRe.ReplaceAllString(s, ""))
}

// splitText режет текст по границам строк на куски ≤ limit символов.
// Строки длиннее limit режутся по пробелам.
func splitText(s string, limit int) []string {
	var out []string
	var cur strings.Builder
	curLen := 0
	push := func() {
		if t := strings.TrimSpace(cur.String()); t != "" {
			out = append(out, t)
		}
		cur.Reset()
		curLen = 0
	}
	for _, line := range strings.Split(s, "\n") {
		for _, piece := range cutLong(line, limit) {
			n := utf8.RuneCountInString(piece)
			if curLen+n+1 > limit {
				push()
			}
			if curLen > 0 {
				cur.WriteByte('\n')
				curLen++
			}
			cur.WriteString(piece)
			curLen += n
		}
	}
	push()
	return out
}

func cutLong(s string, limit int) []string {
	r := []rune(s)
	var out []string
	for len(r) > limit {
		cut := limit
		for i := limit; i > limit/2; i-- {
			if r[i] == ' ' {
				cut = i
				break
			}
		}
		out = append(out, string(r[:cut]))
		r = []rune(strings.TrimLeft(string(r[cut:]), " "))
	}
	return append(out, string(r))
}
