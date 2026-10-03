package main

// gdz_send.go — команда /gdz и отправка результата разбора в Telegram.
//
// Принимает GDZResult из gdz_parse.go (FetchGDZ) и отправляет его куски по порядку:
// текст — HTML-сообщениями, картинки — альбомами. Картинки скачиваются через
// DownloadImage из reshak.go. Обработчик подключается в initComs (main.go).

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
	maxTextLen    = 3500 // длина одного текстового сообщения (лимит Telegram — 4096, остальное — запас на теги)
	maxAlbum      = 10   // максимум фото в одном sendMediaGroup
	maxCaptionLen = 1000 // длина подписи к фото (лимит Telegram — 1024)
)

// ───────────────────────────── КОМАНДА /gdz ─────────────────────────────

// Регистрирует обработчик /gdz <предмет> <номер>. Разбирает аргументы (resolveGDZ),
// скачивает и разбирает страницу (FetchGDZ), отправляет результат (sendGDZ).
// Ошибки скачивания и разбора пересылает пользователю текстом; без аргументов
// отвечает справкой (gdzUsage).
func gdzCom(bh *th.BotHandler) {
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chID := update.Message.Chat.ChatID()
		reply := func(text string) {
			_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, text))
		}

		fields := strings.Fields(update.Message.Text)
		if len(fields) > 0 {
			fields = fields[1:] // нулевое слово — сама команда (/gdz или /gdz@bot)
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
	}, th.Or(
		th.CommandEqual("gdz"),
		th.TextPrefix("гдз"),
		th.TextPrefix("решение"),
		th.TextPrefix("задание"),
		th.TextPrefix("ответ"),
		
	))
}

// Определяет предмет по первому слову после команды (findGDZSubject), остальные
// слова склеивает в аргумент. Старый формат «/gdz 74», где первое слово — число,
// считается алгеброй. Предмет не нашёлся — возвращает nil.
func resolveGDZ(args []string) (*gdzSubject, string) {
	if len(args) == 0 {
		return nil, ""
	}
	if s, ok := findGDZSubject(args[0]); ok {
		return s, strings.Join(args[1:], " ")
	}
	if reNum.MatchString(args[0]) {
		s, _ := findGDZSubject("алг")
		return s, strings.Join(args, " ")
	}
	return nil, ""
}

// Собирает текст справки по таблице gdzSubjects (gdz_parse.go): строка на предмет
// с примером из поля Usage.
func gdzUsage() string {
	var sb strings.Builder
	sb.WriteString("Формат: /gdz <предмет> <номер>\n\n")
	for _, s := range gdzSubjects {
		fmt.Fprintf(&sb, "/gdz %s — %s\n", s.Usage, s.Name)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// ───────────────────────────── ОТПРАВКА ─────────────────────────────

// Идёт по res.Parts и отправляет куски в исходном порядке. Подряд идущие картинки
// копит и отправляет одним вызовом sendImages, текст режет splitText и шлёт через
// sendHTML. Заголовок страницы идёт подписью к первой картинке, а если ответ
// начинается с текста — жирной первой строкой. Возвращает ошибку, если часть
// картинок не доставлена.
func sendGDZ(ctx *th.Context, chID telego.ChatID, res GDZResult) error {
	caption, textHead := "", ""
	if title := strings.TrimSpace(res.Title); title != "" {
		if len(res.Parts) > 0 && res.Parts[0].Image != "" {
			caption = title
		} else {
			textHead = "<b>" + html.EscapeString(title) + "</b>\n\n"
		}
	}

	var pending []string // картинки, ещё не отправленные
	failed := 0
	flushImages := func() {
		if len(pending) > 0 {
			failed += sendImages(ctx, chID, pending, res.URL, caption)
			caption, pending = "", nil // подпись нужна только первой группе
		}
	}

	for _, p := range res.Parts {
		if p.Image != "" {
			pending = append(pending, p.Image)
			continue
		}
		flushImages()
		for _, chunk := range splitText(textHead+p.Text, maxTextLen) {
			sendHTML(ctx, chID, chunk)
		}
		textHead = "" // заголовок выводится один раз, перед первым текстом
	}
	flushImages()

	if failed > 0 {
		return fmt.Errorf("не удалось отправить картинок: %d", failed)
	}
	return nil
}

// Отправляет одно сообщение с parse_mode=HTML. Если Telegram отверг разметку
// (например, тег оказался разрезан), повторяет то же сообщение обычным текстом
// без тегов (plainText). После отправки ждёт 300 мс, чтобы не упереться в лимиты.
func sendHTML(ctx *th.Context, chID telego.ChatID, text string) {
	if _, err := ctx.Bot().SendMessage(ctx, tu.Message(chID, text).WithParseMode(telego.ModeHTML)); err != nil {
		log.Println("gdz: sendMessage(HTML):", err)
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(chID, plainText(text)))
	}
	time.Sleep(300 * time.Millisecond)
}

// Скачанная картинка: имя файла для Telegram и её байты.
type gdzFile struct {
	name string
	data []byte
}

// Скачивает картинки по адресам (DownloadImage из reshak.go; Telegram сам с
// reshak.ru не скачает), нарезает на группы по maxAlbum и отправляет каждую
// через sendAlbum. Подпись получает только первая группа. Возвращает число
// картинок, которые не удалось ни скачать, ни отправить.
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

	caption = plainText(caption)
	if r := []rune(caption); len(r) > maxCaptionLen {
		caption = string(r[:maxCaptionLen])
	}

	for start := 0; start < len(files); start += maxAlbum {
		failed += sendAlbum(ctx, chID, files[start:min(start+maxAlbum, len(files))], caption)
		caption = ""
		time.Sleep(400 * time.Millisecond)
	}
	return failed
}

// Отправляет группу картинок одним альбомом (sendMediaGroup), подпись ставит на
// первую. Если в группе одна картинка или Telegram отклонил альбом (одна плохая
// картинка роняет все), шлёт картинки по одной через sendOne. Возвращает число
// неотправленных.
func sendAlbum(ctx *th.Context, chID telego.ChatID, files []gdzFile, caption string) (failed int) {
	if len(files) > 1 {
		media := make([]telego.InputMedia, 0, len(files))
		for i, f := range files {
			m := tu.MediaPhoto(tu.FileFromReader(bytes.NewReader(f.data), f.name))
			if i == 0 && caption != "" {
				m = m.WithCaption(caption)
			}
			media = append(media, m)
		}
		_, err := ctx.Bot().SendMediaGroup(ctx, tu.MediaGroup(chID, media...))
		if err == nil {
			return 0
		}
		log.Println("gdz: sendMediaGroup:", err)
	}

	for i, f := range files {
		c := ""
		if i == 0 {
			c = caption
		}
		if sendOne(ctx, chID, f, c) != nil {
			failed++
		}
		if i < len(files)-1 {
			time.Sleep(300 * time.Millisecond)
		}
	}
	return failed
}

// Отправляет одну картинку как фото. Если Telegram фото не принял (слишком
// длинная или узкая картинка), отправляет тот же файл документом. Возвращает
// ошибку, только если не получилось и это.
func sendOne(ctx *th.Context, chID telego.ChatID, f gdzFile, caption string) error {
	photo := tu.Photo(chID, tu.FileFromReader(bytes.NewReader(f.data), f.name))
	if caption != "" {
		photo = photo.WithCaption(caption)
	}
	_, err := ctx.Bot().SendPhoto(ctx, photo)
	if err == nil {
		return nil
	}
	log.Println("gdz: sendPhoto:", f.name, err)

	doc := tu.Document(chID, tu.FileFromReader(bytes.NewReader(f.data), f.name)) // читатель нужен новый: прошлый уже прочитан
	if caption != "" {
		doc = doc.WithCaption(caption)
	}
	if _, err := ctx.Bot().SendDocument(ctx, doc); err != nil {
		log.Println("gdz: sendDocument:", f.name, err)
		return err
	}
	return nil
}

// ───────────────────────────── УТИЛИТЫ ─────────────────────────────

// Берёт расширение из пути адреса картинки (png, jpg, jpeg, webp, gif);
// если расширение другое или адрес не разобрался — возвращает ".png".
func imgExt(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		switch ext := strings.ToLower(path.Ext(u.Path)); ext {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif":
			return ext
		}
	}
	return ".png"
}

// Вырезает из HTML-строки теги (tagRe из gdz_parse.go) и превращает &amp;, &#39;
// и подобное обратно в обычные символы. Нужна для подписей и для повторной
// отправки текста без разметки.
func plainText(s string) string {
	return html.UnescapeString(tagRe.ReplaceAllString(s, ""))
}

// Режет текст на куски не длиннее limit символов, склеивая целые строки.
// Границы проходят только между строками (walker из gdz_parse.go нигде не
// оставляет открытые теги на конце строки); строки, которые длиннее limit,
// предварительно режет cutLong.
func splitText(s string, limit int) []string {
	var out, cur []string // готовые куски; строки куска, который собирается
	curLen := 0           // длина cur с учётом переносов между строками
	flush := func() {
		if t := strings.TrimSpace(strings.Join(cur, "\n")); t != "" {
			out = append(out, t)
		}
		cur, curLen = nil, 0
	}
	for _, line := range strings.Split(s, "\n") {
		for _, piece := range cutLong(line, limit) {
			n := utf8.RuneCountInString(piece)
			if curLen+n+1 > limit {
				flush()
			}
			if len(cur) > 0 {
				curLen++ // перенос строки перед piece
			}
			cur = append(cur, piece)
			curLen += n
		}
	}
	flush()
	return out
}

// Режет одну длинную строку на части не длиннее limit символов. Место разреза
// ищет по последнему пробелу во второй половине допустимой длины; если пробела
// нет — режет ровно по limit. Пробелы в начале каждой следующей части убирает.
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
