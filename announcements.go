package main

// announcements.go — оповещения: хранение в .md и ветки админ-панели
// announcement:make / drafts / publish.
//
// Файл announcements/drafts/<тип>_<ДД-ММ-ГГГГ>[-<n>].md:
//
//	---
//	type: announcement
//	created: 2026-10-20T14:35:10+10:00
//	words: 24
//	---
//
//	# *Заголовок ![😇](tg://emoji?id=5906995262378741881)*
//
//	Текст в MarkdownV2, кастомные эмодзи — как ![😇](tg://emoji?id=...)\.
//
// Заголовок и текст хранятся в MarkdownV2 (см. tgmd.go) и уходят в Telegram как есть.
// Опубликованное переносится в announcements/archive/.

import (
	"errors"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Key попадает в имя файла (только a-z, до 12 букв), Want решает, кому рассылать.
type announceType struct {
	Key   string
	Title string
	Want  func(Classmate) bool
}

var announceTypes = []announceType{
	{"announcement", "Нововведения", func(c Classmate) bool { return c.Innovations }},
}

func announceTypeByKey(key string) (announceType, bool) {
	i := slices.IndexFunc(announceTypes, func(t announceType) bool { return t.Key == key })
	if i < 0 {
		return announceType{}, false
	}
	return announceTypes[i], true
}

var (
	annDir       = "announcements" // флаг --announcements-dir
	annMu        sync.Mutex        // имена файлов и перемещения
	annPendingMu sync.Mutex
	annPending   = map[int64]announcement{} // созданное, но ещё не нажатое «Закончить»; ключ — чат админа
	annNameRe    = regexp.MustCompile(`^[a-z]+_\d{2}-\d{2}-\d{4}(-\d+)?$`)
)

func draftsDir() string              { return filepath.Join(annDir, "drafts") }
func archiveDir() string             { return filepath.Join(annDir, "archive") }
func draftPath(stem string) string   { return filepath.Join(draftsDir(), stem+".md") }
func archivePath(stem string) string { return filepath.Join(archiveDir(), stem+".md") }

func isAdmin(id int64) bool {
	c, ok := Groups.Get(id)
	return id == ownerID || (ok && c.Admin)
}

// ───────────────────────────── МОДЕЛЬ И ФАЙЛЫ ─────────────────────────────

type announcement struct {
	Type    string
	Created time.Time
	Words   int    // слов в заголовке и тексте вместе
	Title   string // MarkdownV2
	Body    string // MarkdownV2
}

// То, что получит пользователь.
func (a announcement) text() string { return a.Title + "\n\n" + a.Body }

func (a announcement) markdown() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "---\ntype: %s\ncreated: %s\nwords: %d\n---\n\n# %s\n\n%s\n",
		a.Type, a.Created.Format(time.RFC3339), a.Words, a.Title, a.Body)
	return sb.String()
}

func parseAnnouncement(data string) (announcement, error) {
	data = strings.ReplaceAll(data, "\r\n", "\n")
	rest, ok := strings.CutPrefix(data, "---\n")
	if !ok {
		return announcement{}, fmt.Errorf("нет заголовка файла")
	}
	meta, rest, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return announcement{}, fmt.Errorf("не закрыт заголовок файла")
	}
	var a announcement
	for _, line := range strings.Split(meta, "\n") {
		k, v, _ := strings.Cut(line, ": ")
		switch k {
		case "type":
			a.Type = v
		case "created":
			a.Created, _ = time.Parse(time.RFC3339, v)
		case "words":
			a.Words, _ = strconv.Atoi(v)
		}
	}
	rest = strings.TrimLeft(rest, "\n")
	first, body, _ := strings.Cut(rest, "\n\n")
	a.Title, ok = strings.CutPrefix(first, "# ")
	if !ok {
		return announcement{}, fmt.Errorf("нет строки с заголовком")
	}
	a.Body = strings.TrimRight(body, "\n")
	return a, nil
}

func annExists(stem string) bool {
	for _, p := range []string{draftPath(stem), archivePath(stem)} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// Имя: тип_ДД-ММ-ГГГГ, а если такое уже есть (в черновиках или архиве) — с суффиксом -1, -2, …
func saveDraft(a announcement) (string, error) {
	annMu.Lock()
	defer annMu.Unlock()
	for _, dir := range []string{draftsDir(), archiveDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	base := a.Type + "_" + a.Created.Format("02-01-2006")
	stem := base
	for n := 1; annExists(stem); n++ {
		stem = fmt.Sprintf("%s-%d", base, n)
	}
	tmp := draftPath(stem) + ".tmp"
	if err := os.WriteFile(tmp, []byte(a.markdown()), 0o644); err != nil {
		return "", err
	}
	return stem, os.Rename(tmp, draftPath(stem))
}

func loadDraft(stem string) (announcement, error) {
	if !annNameRe.MatchString(stem) {
		return announcement{}, fmt.Errorf("некорректное имя")
	}
	data, err := os.ReadFile(draftPath(stem))
	if err != nil {
		return announcement{}, err
	}
	return parseAnnouncement(string(data))
}

// Удаляет и из черновиков, и из архива.
func deleteAnnouncement(stem string) error {
	if !annNameRe.MatchString(stem) {
		return fmt.Errorf("некорректное имя")
	}
	annMu.Lock()
	defer annMu.Unlock()
	found := false
	for _, p := range []string{draftPath(stem), archivePath(stem)} {
		err := os.Remove(p)
		if err == nil {
			found = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if !found {
		return os.ErrNotExist
	}
	return nil
}

func parseStem(stem string) (date time.Time, n int, ok bool) {
	if !annNameRe.MatchString(stem) {
		return time.Time{}, 0, false
	}
	_, rest, _ := strings.Cut(stem, "_")
	d, err := time.Parse("02-01-2006", rest[:10])
	if err != nil {
		return time.Time{}, 0, false
	}
	if len(rest) > 10 {
		n, _ = strconv.Atoi(rest[11:])
	}
	return d, n, true
}

// «20.10», а для второго и следующих за день — «20.10 №2».
func annLabel(stem string) string {
	date, n, ok := parseStem(stem)
	if !ok {
		return stem
	}
	l := date.Format("02.01")
	if n > 0 {
		l += fmt.Sprintf(" №%d", n+1)
	}
	return l
}

func listDrafts() []string {
	entries, err := os.ReadDir(draftsDir())
	if err != nil {
		return nil
	}
	var stems []string
	for _, e := range entries {
		if stem, ok := strings.CutSuffix(e.Name(), ".md"); ok {
			if _, _, ok := parseStem(stem); ok {
				stems = append(stems, stem)
			}
		}
	}
	slices.SortFunc(stems, func(a, b string) int {
		da, na, _ := parseStem(a)
		db, nb, _ := parseStem(b)
		if c := da.Compare(db); c != 0 {
			return c
		}
		return na - nb
	})
	return stems
}

func recipients(typ string) []int64 {
	t, ok := announceTypeByKey(typ)
	if !ok {
		return nil
	}
	var out []int64
	for _, id := range Groups.IDs() {
		if id <= 0 {
			continue
		}
		if c, ok := Groups.Get(id); ok && t.Want(c) {
			out = append(out, id)
		}
	}
	return out
}

// Сначала файл переносится в архив (повторное нажатие «Опубликовать» уже ничего не найдёт),
// потом идёт рассылка.
func publishAnnouncement(ctx *th.Context, stem string, a announcement) (sent, total int, err error) {
	annMu.Lock()
	err = os.Rename(draftPath(stem), archivePath(stem))
	annMu.Unlock()
	if err != nil {
		return 0, 0, fmt.Errorf("уже опубликовано или удалено")
	}
	ids := recipients(a.Type)
	text := a.text()
	for _, id := range ids {
		if _, e := ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(id), text).WithParseMode(telego.ModeMarkdownV2)); e != nil {
			log.Println("рассылка оповещения:", id, e)
		} else {
			sent++
		}
		time.Sleep(60 * time.Millisecond) // лимит Telegram — около 30 сообщений в секунду
	}
	return sent, len(ids), nil
}

func takePending(chat int64) (announcement, bool) {
	annPendingMu.Lock()
	defer annPendingMu.Unlock()
	a, ok := annPending[chat]
	delete(annPending, chat)
	return a, ok
}

func putPending(chat int64, a announcement) {
	annPendingMu.Lock()
	annPending[chat] = a
	annPendingMu.Unlock()
}

func peopleWord(n int) string {
	if n%100 >= 11 && n%100 <= 14 {
		return "человек"
	}
	switch n % 10 {
	case 2, 3, 4:
		return "человека"
	}
	return "человек"
}

// ───────────────────────────── ВАЛИДАЦИЯ И СБОРКА ─────────────────────────────

func validateAnnTitle(m *telego.Message) error {
	switch {
	case strings.TrimSpace(m.Text) == "":
		return fmt.Errorf("нужен текст заголовка")
	case strings.Contains(m.Text, "\n"):
		return fmt.Errorf("заголовок должен быть в одну строку")
	}
	if n := len(strings.Fields(m.Text)); n >= 16 {
		return fmt.Errorf("в заголовке %d слов, нужно меньше 16", n)
	}
	return nil
}

func validateAnnBody(title, m *telego.Message) error {
	if strings.TrimSpace(m.Text) == "" {
		return fmt.Errorf("нужен текст оповещения")
	}
	if utf16Len(title.Text)+2+utf16Len(m.Text) > 4096 {
		return fmt.Errorf("вместе с заголовком длиннее 4096 символов")
	}
	return nil
}

func buildAnnouncement(key string, title, body *telego.Message) announcement {
	tents := slices.Clone(title.Entities)
	if !slices.ContainsFunc(tents, func(e telego.MessageEntity) bool { return isCodeEnt(e.Type) }) {
		// заголовок всегда жирный; пересечения с уже жирным кусочками склеит конвертер
		tents = append(tents, telego.MessageEntity{Type: telego.EntityTypeBold, Offset: 0, Length: utf16Len(title.Text)})
	}
	return announcement{
		Type:  key,
		Words: len(strings.Fields(title.Text)) + len(strings.Fields(body.Text)),
		Title: entitiesToMarkdownV2(title.Text, tents),
		Body:  entitiesToMarkdownV2(body.Text, body.Entities),
	}
}

// ───────────────────────────── СОЗДАНИЕ ─────────────────────────────

// announcement:make[:тип|ok|del]
func announcementMake(ctx *th.Context, query telego.CallbackQuery, p []string) {
	d := newAdminDlg(ctx, query, "announcement")
	chat := d.chatID.ID

	// 1. тема
	if len(p) == 0 {
		var rows [][]telego.InlineKeyboardButton
		for _, t := range announceTypes {
			rows = append(rows, tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.Title).WithCallbackData("announcement:make:"+t.Key).WithStyle("primary")))
		}
		d.show("Выбери тему оповещения", rows...)
		return
	}

	// кнопки под готовым оповещением
	switch p[0] {
	case "ok":
		a, ok := takePending(chat)
		if !ok {
			d.show("Черновик потерян (бот перезапускался). Создай оповещение заново")
			return
		}
		a.Created = time.Now()
		if _, err := saveDraft(a); err != nil {
			putPending(chat, a)
			d.show("Не удалось сохранить: " + html.EscapeString(err.Error()))
			return
		}
		adminpanel(query, ctx, 2)
		return
	case "del":
		takePending(chat)
		adminpanel(query, ctx, 2)
		return
	}

	t, ok := announceTypeByKey(p[0])
	if !ok {
		return
	}

	// 2. заголовок
	title := d.askMsgUntil(func(errLine string) {
		d.show(errLine + "Напиши заголовок (меньше 16 слов). Форматирование и кастомные эмодзи сохранятся")
	}, validateAnnTitle)
	if title == nil {
		return
	}

	// 3. текст. Валидатор заодно показывает результат: сообщение диалога превращается
	// в готовое оповещение с кнопками. Если Telegram не принял разметку, админ увидит ошибку и пришлёт текст снова.
	body := d.askMsgUntil(func(errLine string) {
		d.show(errLine + "Теперь напиши текст оповещения")
	}, func(m *telego.Message) error {
		if err := validateAnnBody(title, m); err != nil {
			return err
		}
		draft := buildAnnouncement(t.Key, title, m)
		putPending(chat, draft)
		_, err := ctx.Bot().EditMessageText(ctx, tu.EditMessageText(d.chatID, d.msgID, draft.text()).
			WithParseMode(telego.ModeMarkdownV2).
			WithReplyMarkup(tu.InlineKeyboard(
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Закончить").WithCallbackData("announcement:make:ok").WithStyle("success")),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Удалить").WithCallbackData("announcement:make:del").WithStyle("danger")),
			)))
		if err != nil {
			takePending(chat)
			return fmt.Errorf("telegram не принял разметку (%v)", err)
		}
		return nil
	})
	_ = body // всё нужное уже лежит в annPending
}

// ───────────────────────────── СПИСОК И ПУБЛИКАЦИЯ ─────────────────────────────

func announcementList(ctx *th.Context, query telego.CallbackQuery, title string) {
	d := newAdminDlg(ctx, query, "announcement")
	stems := listDrafts()
	if len(stems) == 0 {
		d.show("Черновиков нет")
		return
	}
	if len(stems) > 30 {
		stems = stems[len(stems)-30:] // самые свежие
	}
	var btns []telego.InlineKeyboardButton
	for _, s := range stems {
		btns = append(btns, tu.InlineKeyboardButton(annLabel(s)).
			WithCallbackData("announcement:publish:"+s).WithStyle("primary"))
	}
	var rows [][]telego.InlineKeyboardButton
	for c := range slices.Chunk(btns, 3) {
		rows = append(rows, tu.InlineKeyboardRow(c...))
	}
	d.show(html.EscapeString(title), rows...)
}

// announcement:publish:<имя>[:go|:rm[:y]]
func announcementItem(ctx *th.Context, query telego.CallbackQuery, p []string) {
	d := newAdminDlg(ctx, query, "announcement")
	stem := p[0]
	a, err := loadDraft(stem)
	if err != nil {
		d.show("Оповещение не найдено: возможно, оно уже опубликовано или удалено")
		return
	}
	pre := "announcement:publish:" + stem
	act := ""
	if len(p) > 1 {
		act = p[1]
	}

	switch act {
	case "go":
		d.show("Рассылаю…")
		sent, total, err := publishAnnouncement(ctx, stem, a)
		if err != nil {
			d.show("Не удалось опубликовать: " + html.EscapeString(err.Error()))
			return
		}
		d.show(fmt.Sprintf("Опубликовано: доставлено %d из %d. Оповещение перенесено в архив", sent, total))

	case "rm":
		if len(p) > 2 && p[2] == "y" {
			if err := deleteAnnouncement(stem); err != nil {
				d.show("Не удалось удалить: " + html.EscapeString(err.Error()))
				return
			}
			announcementList(ctx, query, "Черновики")
			return
		}
		d.show(fmt.Sprintf("Удалить оповещение %s навсегда?", annLabel(stem)),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Да, удалить").WithCallbackData(pre+":rm:y").WithStyle("danger"),
				tu.InlineKeyboardButton("Нет").WithCallbackData(pre).WithStyle("success"),
			))

	default:
		typeTitle := a.Type
		if t, ok := announceTypeByKey(a.Type); ok {
			typeTitle = t.Title
		}
		n := len(recipients(a.Type))
		created := fmt.Sprintf("%s %d, %s", formatHeaderDate(a.Created), a.Created.Year(), a.Created.Format("15:04"))
		txt, ents := tu.MessageEntities(
			tu.Entity("🍎").CustomEmoji("5818920837645867167"), tu.Entity(" Оповещение: ").Bold(), tu.Entity(typeTitle), tu.Entity("\n"),
			tu.Entity("🏷").CustomEmoji("5987802868734760945"), tu.Entity(" Слов с заголовком: ").Bold(), tu.Entityf("%d\n", a.Words),
			tu.Entity("🗒").CustomEmoji("5877597667231534929"), tu.Entity(" Создано: ").Bold(), tu.Entity(created), tu.Entity("\n"),
			tu.Entity("👤").CustomEmoji("5879770735999717115"), tu.Entity(" Получат: ").Bold(), tu.Entityf("%d %s", n, peopleWord(n)),
		)
		ctx.Bot().EditMessageText(ctx, tu.EditMessageText(d.chatID, d.msgID, txt).
			WithEntities(ents...).
			WithReplyMarkup(tu.InlineKeyboard(
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Опубликовать").WithCallbackData(pre+":go").WithStyle("success")),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Отмена").WithCallbackData("announcement:drafts").WithStyle("primary")),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Удалить").WithCallbackData(pre+":rm").WithStyle("danger")),
			)))
	}
}
