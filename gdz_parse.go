package main

// gdz_parse.go — парсеры страниц reshak.ru по предметам.
//
// Слой разбора не знает про Telegram: он превращает страницу в GDZResult —
// упорядоченный список кусочков, каждый из которых либо текст (HTML для
// parse_mode=HTML), либо URL картинки. Отправка — в gdz_send.go.

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

// GDZPart — один кусочек ответа: либо Text, либо Image (абсолютный URL).
type GDZPart struct {
	Text  string
	Image string
}

// GDZResult — разобранная страница решебника.
type GDZResult struct {
	Title string // заголовок страницы (обычный текст, без HTML)
	URL   string // откуда взято (нужен как Referer при скачивании картинок)
	Parts []GDZPart
}

var errGDZEmpty = errors.New("на странице нет ответа (возможно, такого номера нет в решебнике)")

// ───────────────────────────── ПРЕДМЕТЫ ─────────────────────────────

type gdzSubject struct {
	Name    string
	Aliases []string
	Usage   string // пример аргументов для справки
	URL     func(arg string) (string, error)
	Parse   func(doc *goquery.Document, base *url.URL) (GDZResult, error)
}

var gdzSubjects = []gdzSubject{
	{"Алгебра (Алимов)", []string{"алг", "алгебра", "алимов"}, "алг 74", algebraURL, parseImages},
	{"Геометрия (Атанасян)", []string{"геом", "геометрия", "атанасян"}, "геом 222", geometryURL, parseImages},
	{"Русский язык (Рыбченкова)", []string{"рус", "русс", "русский"}, "рус 50", russianURL, parseImages},
	{"Физика (Мякишев)", []string{"физ", "физика", "мякишев"}, "физ 21-1", physicsURL, parsePhysics},
	{"Химия (Габриелян)", []string{"хим", "химия"}, "хим 4-1", chemistryURL, parseChemistry},
	{"Литература (Лебедев)", []string{"лит", "литра", "литература", "лебедев"}, "лит 1/134", literatureURL, parseLiterature},
	{"История (Мединский)", []string{"ист", "история"}, "ист 1", historyURL, parseHistory},
	{"Английский (Forward)", []string{"англ", "английский", "forward"}, "англ 8-9", englishURL, parseEnglish},
}

func findGDZSubject(word string) (*gdzSubject, bool) {
	word = strings.ToLower(strings.TrimSpace(word))
	for i := range gdzSubjects {
		for _, a := range gdzSubjects[i].Aliases {
			if a == word {
				return &gdzSubjects[i], true
			}
		}
	}
	return nil, false
}

// ───────────────────────────── URL-БИЛДЕРЫ ─────────────────────────────
// Аргументы пользователя подставляются в URL, поэтому каждый проверяется
// регуляркой — никаких «&predmet=…» через чат.

var (
	reNum      = regexp.MustCompile(`^\d{1,4}$`)
	reNumDash  = regexp.MustCompile(`^\d{1,3}-\d{1,3}$`)
	reNumRange = regexp.MustCompile(`^\d{1,3}(-\d{1,3})?$`)
	rePartPage = regexp.MustCompile(`^([12])[/ ](\d{1,3})$`)
)

func reshebnik(predmet, otvet string) string {
	return fmt.Sprintf("%s/otvet/reshebniki.php?otvet=%s&predmet=%s", reshakBase, otvet, predmet)
}

func badArg(usage string) error {
	return fmt.Errorf("неверный номер, пример: %s", usage)
}

func algebraURL(a string) (string, error) {
	if !reNum.MatchString(a) {
		return "", badArg("/gdz алг 74")
	}
	return fmt.Sprintf("%s/otvet/otvet15.php?otvet=%s", reshakBase, a), nil
}

func geometryURL(a string) (string, error) {
	if !reNum.MatchString(a) {
		return "", badArg("/gdz геом 222")
	}
	return reshebnik("atan10_11", "new/"+a), nil
}

func russianURL(a string) (string, error) {
	if !reNum.MatchString(a) {
		return "", badArg("/gdz рус 50")
	}
	return reshebnik("ribchenkova10-11", a), nil
}

func physicsURL(a string) (string, error) { // «параграф-задание»
	if !reNumDash.MatchString(a) {
		return "", badArg("/gdz физ 21-1 (параграф-задание)")
	}
	return reshebnik("myakishev10", a), nil
}

func chemistryURL(a string) (string, error) { // «параграф-вопрос»
	if !reNumDash.MatchString(a) {
		return "", badArg("/gdz хим 4-1 (параграф-вопрос)")
	}
	return reshebnik("ostroumov10", a), nil
}

func literatureURL(a string) (string, error) { // «часть/страница»
	m := rePartPage.FindStringSubmatch(a)
	if m == nil {
		return "", badArg("/gdz лит 1/134 (часть/страница)")
	}
	return reshebnik("lebedev_baz10", "part"+m[1]+"/"+m[2]), nil
}

func historyURL(a string) (string, error) { // номер параграфа
	if !reNum.MatchString(a) {
		return "", badArg("/gdz ист 1 (номер параграфа)")
	}
	return reshebnik("medinsky_vseobshaya10", a), nil
}

func englishURL(a string) (string, error) { // страницы: «8-9» или «12»
	if !reNumRange.MatchString(a) {
		return "", badArg("/gdz англ 8-9")
	}
	return fmt.Sprintf("%s/otvet/otvet_txt.php?otvet1=/forward10/images/%s", reshakBase, a), nil
}

// ───────────────────────────── ЗАГРУЗКА ─────────────────────────────

// FetchGDZ скачивает страницу предмета и разбирает её.
func FetchGDZ(ctx context.Context, s *gdzSubject, arg string) (GDZResult, error) {
	pageURL, err := s.URL(strings.TrimSpace(arg))
	if err != nil {
		return GDZResult{}, err
	}
	resp, err := reshakGet(ctx, pageURL, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "")
	if err != nil {
		return GDZResult{}, err
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return GDZResult{}, err
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return GDZResult{}, err
	}
	res, err := s.Parse(doc, base)
	res.URL = pageURL
	return res, err
}

// ───────────────────────────── ПАРСЕРЫ СТРАНИЦ ─────────────────────────────

func newResult(doc *goquery.Document) GDZResult {
	return GDZResult{Title: collapseSpaces(strings.TrimSpace(doc.Find("h1.titleh1").First().Text()))}
}

func finish(res GDZResult) (GDZResult, error) {
	if len(res.Parts) == 0 {
		return res, errGDZEmpty
	}
	return res, nil
}

// Алгебра, геометрия, русский: ответ — только картинки (решение №1, №2 и доп.).
func parseImages(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	res.Parts = pageImages(doc, base)
	return finish(res)
}

// Физика: картинки решений + (если есть) текстовый «ИИ-разбор».
func parsePhysics(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	res.Parts = pageImages(doc, base)
	if ai := doc.Find(".ai-analysis__answer").First(); ai.Length() > 0 {
		w := newWalker(base)
		w.walk(ai)
		w.flush()
		if txt := joinText(w.parts); txt != "" {
			res.Parts = append(res.Parts, GDZPart{Text: "<b>ИИ-разбор</b>\n" + txt})
		}
	}
	return finish(res)
}

// Химия: картинка решения + текст условия (блок .text_zad без служебной шапки).
func parseChemistry(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	res.Parts = pageImages(doc, base)
	if zad := doc.Find("article.lcol .text_zad").First(); zad.Length() > 0 {
		w := newWalker(base)
		w.walk(zad)
		w.flush()
		if txt := dropBoilerplate(joinText(w.parts)); txt != "" {
			res.Parts = append(res.Parts, GDZPart{Text: "<b>Условие</b>\n" + txt})
		}
	}
	return finish(res)
}

// Литература: много картинок (на стр. 134 их 35) + ссылка на презентацию, если есть.
// Блок .text_zad здесь — это ~45 000 символов текста, его намеренно НЕ берём.
func parseLiterature(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	res.Parts = pageImages(doc, base)
	doc.Find("article.lcol fieldset.present a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		if strings.HasPrefix(href, "http") {
			res.Parts = append(res.Parts, GDZPart{
				Text: fmt.Sprintf(`📎 Презентация: <a href="%s">ссылка</a>`, html.EscapeString(href)),
			})
		}
	})
	return finish(res)
}

// История: сплошной текст (вопросы выделены жирным, есть таблица) с картинками
// прямо посреди текста — порядок сохраняется.
func parseHistory(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	root := doc.Find("article.lcol .mainInfo").First()
	if root.Length() == 0 {
		return res, errGDZEmpty
	}
	w := newWalker(base)
	w.walk(root)
	w.flush()
	res.Parts = dedupeImages(w.parts)
	return finish(res)
}

// Английский (Forward): только текст, оригинал + перевод курсивом.
func parseEnglish(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	root := doc.Find("article.lcol .mainInfo").First()
	if root.Length() == 0 {
		return res, errGDZEmpty
	}
	w := newWalker(base)
	w.walk(root)
	w.flush()
	for _, p := range w.parts {
		if p.Text != "" {
			p.Text = strings.ReplaceAll(p.Text, "Решение #", "Решение")
		}
		res.Parts = append(res.Parts, p)
	}
	return finish(res)
}

// ───────────────────────────── ХЕЛПЕРЫ ─────────────────────────────

// pageImages — все картинки решений в порядке документа:
// pic_otvet1, pic_otvet1_dop1…N, pic_otvet2, pic_otvet2_dop1…N.
func pageImages(doc *goquery.Document, base *url.URL) []GDZPart {
	var out []GDZPart
	seen := map[string]bool{}
	doc.Find("article.lcol div[class^='pic_otvet'] img").Each(func(_ int, s *goquery.Selection) {
		u := imgURL(s, base)
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, GDZPart{Image: u})
	})
	return out
}

// imgURL: lazyload кладёт настоящий адрес в data-src, обычные картинки — в src.
func imgURL(s *goquery.Selection, base *url.URL) string {
	for _, attr := range []string{"data-src", "data-original", "src"} {
		v, ok := s.Attr(attr)
		v = strings.TrimSpace(v)
		if !ok || v == "" || strings.HasPrefix(v, "data:") {
			continue
		}
		ref, err := url.Parse(v)
		if err != nil {
			continue
		}
		full := base.ResolveReference(ref)
		if full.Scheme == "http" || full.Scheme == "https" {
			return full.String()
		}
	}
	return ""
}

func dedupeImages(parts []GDZPart) []GDZPart {
	seen := map[string]bool{}
	out := parts[:0:0]
	for _, p := range parts {
		if p.Image != "" {
			if seen[p.Image] {
				continue
			}
			seen[p.Image] = true
		}
		out = append(out, p)
	}
	return out
}

func joinText(parts []GDZPart) string {
	var sb []string
	for _, p := range parts {
		if p.Text != "" {
			sb = append(sb, p.Text)
		}
	}
	return strings.TrimSpace(strings.Join(sb, "\n"))
}

var boilerplateRe = regexp.MustCompile(`^(Рассмотрим вариант решения|Приведем выдержку)`)

// dropBoilerplate убирает строки вида «Рассмотрим вариант решения задания из учебника…».
func dropBoilerplate(text string) string {
	var keep []string
	for _, line := range strings.Split(text, "\n") {
		if boilerplateRe.MatchString(tagRe.ReplaceAllString(line, "")) {
			continue
		}
		keep = append(keep, line)
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}

func collapseSpaces(s string) string {
	var sb strings.Builder
	sp := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !sp {
				sb.WriteByte(' ')
			}
			sp = true
			continue
		}
		sp = false
		sb.WriteRune(r)
	}
	return sb.String()
}

// ───────────────────────────── HTML → Telegram-HTML ─────────────────────────────
//
// walker обходит DOM и собирает строки. Из форматирования оставляем только
// <b> (strong, заголовки, .question) и <i> (i, em) — их понимает parse_mode=HTML.
// Теги никогда не пересекают границу строки: при сбросе строки открытые теги
// закрываются и открываются заново в следующей. Это нужно, чтобы потом можно
// было резать текст по \n на сообщения ≤ 4096 символов, не ломая разметку.

var tagRe = regexp.MustCompile(`<[^>]+>`)

var blockTags = map[string]bool{
	"div": true, "p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "table": true, "tbody": true, "thead": true, "tr": true,
	"section": true, "article": true, "figure": true, "figcaption": true, "blockquote": true,
	"pre": true, "fieldset": true, "legend": true,
}

type walker struct {
	base    *url.URL
	parts   []GDZPart
	line    strings.Builder
	visible bool // в текущей строке уже есть видимый текст
	lastSp  bool
	stack   []string // открытые теги форматирования
	count   map[string]int
	inCell  int // >0: внутри ячейки таблицы — блоки не рвут строку
	cellIdx int
	gap     bool // перед следующей строкой нужна пустая строка
}

func newWalker(base *url.URL) *walker {
	return &walker{base: base, count: map[string]int{}}
}

func (w *walker) walk(s *goquery.Selection) {
	s.Contents().Each(func(_ int, n *goquery.Selection) { w.node(n) })
}

func (w *walker) node(n *goquery.Selection) {
	name := goquery.NodeName(n)
	switch name {
	case "#text":
		w.text(n.Text())
		return
	case "#comment":
		return
	}
	if skipNode(n, name) {
		return
	}
	switch name {
	case "br":
		w.flush()
		return
	case "img":
		if u := imgURL(n, w.base); u != "" {
			w.flush()
			w.parts = append(w.parts, GDZPart{Image: u})
		}
		return
	}

	isHead := len(name) == 2 && name[0] == 'h' && name[1] >= '1' && name[1] <= '6'
	isQ := n.HasClass("question")
	fmtTag := ""
	switch {
	case name == "b" || name == "strong" || isHead || isQ:
		fmtTag = "b"
	case name == "i" || name == "em":
		fmtTag = "i"
	}

	block := blockTags[name] && w.inCell == 0
	if block {
		w.flush()
	}
	if isHead || isQ {
		w.flush()
		w.gap = true
	}
	switch name {
	case "tr":
		w.cellIdx = 0
	case "td", "th":
		if w.cellIdx > 0 {
			w.text(" | ")
		}
		w.cellIdx++
		w.inCell++
	}

	if fmtTag != "" {
		w.open(fmtTag)
	}
	w.walk(n)
	if fmtTag != "" {
		w.close(fmtTag)
	}

	if name == "td" || name == "th" {
		w.inCell--
	}
	if block {
		w.flush()
	}
}

func skipNode(n *goquery.Selection, name string) bool {
	switch name {
	case "script", "style", "noscript", "noindex", "iframe", "button", "form", "nav", "svg", "audio", "video":
		return true
	}
	if id, _ := n.Attr("id"); strings.HasPrefix(id, "yandex_rtb") {
		return true
	}
	for _, c := range []string{"empty_place", "readmore-js-toggle", "text-zad-note"} {
		if n.HasClass(c) {
			return true
		}
	}
	if st, _ := n.Attr("style"); strings.Contains(strings.ReplaceAll(st, " ", ""), "display:none") {
		return true
	}
	return false
}

func (w *walker) text(s string) {
	s = collapseSpaces(s)
	if s == "" {
		return
	}
	if s[0] == ' ' && (!w.visible || w.lastSp) {
		s = s[1:]
	}
	if s == "" {
		return
	}
	w.line.WriteString(html.EscapeString(s))
	w.visible = true
	w.lastSp = s[len(s)-1] == ' '
}

func (w *walker) open(tag string) {
	if w.count[tag] == 0 {
		w.line.WriteString("<" + tag + ">")
		w.stack = append(w.stack, tag)
	}
	w.count[tag]++
}

func (w *walker) close(tag string) {
	w.count[tag]--
	if w.count[tag] > 0 {
		return
	}
	idx := -1
	for i := len(w.stack) - 1; i >= 0; i-- {
		if w.stack[i] == tag {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	tail := append([]string(nil), w.stack[idx+1:]...)
	for i := len(w.stack) - 1; i >= idx; i-- {
		w.line.WriteString("</" + w.stack[i] + ">")
	}
	for _, t := range tail {
		w.line.WriteString("<" + t + ">")
	}
	w.stack = append(w.stack[:idx], tail...)
}

// flush завершает строку: закрывает открытые теги и открывает их заново в новой строке.
func (w *walker) flush() {
	line := w.line.String()
	w.line.Reset()
	w.visible, w.lastSp = false, false

	closing := ""
	for i := len(w.stack) - 1; i >= 0; i-- {
		closing += "</" + w.stack[i] + ">"
	}
	for _, t := range w.stack {
		w.line.WriteString("<" + t + ">")
	}

	line = strings.TrimSpace(line + closing)
	line = strings.ReplaceAll(line, "</b><b>", "")
	line = strings.ReplaceAll(line, "</i><i>", "")
	if strings.TrimSpace(tagRe.ReplaceAllString(line, "")) == "" {
		return
	}
	if n := len(w.parts); n > 0 && w.parts[n-1].Image == "" {
		sep := "\n"
		if w.gap {
			sep = "\n\n"
		}
		w.parts[n-1].Text += sep + line
	} else {
		w.parts = append(w.parts, GDZPart{Text: line})
	}
	w.gap = false
}
