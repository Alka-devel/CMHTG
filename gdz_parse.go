package main

// gdz_parse.go — разбор HTML-страниц решебника reshak.ru.
//
// Файл ничего не знает про Telegram. На выходе — GDZResult: упорядоченный
// список кусков, где каждый кусок — либо текст в разметке для parse_mode=HTML,
// либо абсолютный URL картинки. Результат дальше уходит в sendGDZ (gdz_send.go),
// а сами страницы скачиваются через reshakGet из reshak.go.

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

// GDZPart — один кусок ответа. Заполнено ровно одно поле: Text или Image.
type GDZPart struct {
	Text  string // HTML для parse_mode=HTML (только <b>, <i>, <a>)
	Image string // абсолютный URL картинки
}

// GDZResult — разобранная страница. Собирается в Parse-функциях предметов,
// URL дописывает FetchGDZ, читает всё это sendGDZ.
type GDZResult struct {
	Title string // содержимое h1.titleh1, обычный текст без HTML
	URL   string // адрес страницы; при скачивании картинок идёт в заголовок Referer
	Parts []GDZPart
}

// Возвращается finish, когда парсер не нашёл на странице ни одного куска.
var errGDZEmpty = errors.New("на странице нет ответа (возможно, такого номера нет в решебнике)")

// ───────────────────────────── ПРЕДМЕТЫ ─────────────────────────────

// Допустимые форматы аргумента команды. Аргумент подставляется прямо в URL,
// поэтому каждый предмет проверяет его одним из этих выражений.
var (
	reNum      = regexp.MustCompile(`^\d{1,4}$`)             // 74
	reNumDash  = regexp.MustCompile(`^\d{1,3}-\d{1,3}$`)     // 21-1
	reNumRange = regexp.MustCompile(`^\d{1,3}(-\d{1,3})?$`)  // 12 или 8-9
	rePartPage = regexp.MustCompile(`^([12])[/ ](\d{1,3})$`) // 1/134 или 1 134
)

// gdzSubject описывает один предмет: по каким словам его находят в команде,
// как проверить аргумент, как собрать URL и каким парсером разбирать страницу.
type gdzSubject struct {
	Name    string
	Aliases []string // слова, которые findGDZSubject принимает как название предмета
	Usage   string   // пример аргументов; показывается в справке и в тексте ошибки
	argRe   *regexp.Regexp
	build   func(arg string) string
	Parse   func(doc *goquery.Document, base *url.URL) (GDZResult, error)
}

// Таблица всех предметов; её читают findGDZSubject, gdzUsage и тесты.
var gdzSubjects = []gdzSubject{
	{
		Name: "Алгебра (Алимов)", Aliases: []string{"алг", "алгебра", "алимов"}, Usage: "алг 74",
		argRe: reNum, Parse: parseImages,
		build: func(a string) string { return fmt.Sprintf("%s/otvet/otvet15.php?otvet=%s", reshakBase, a) },
	},
	{
		Name: "Геометрия (Атанасян)", Aliases: []string{"геом", "геометрия", "атанасян"}, Usage: "геом 222",
		argRe: reNum, Parse: parseImages, build: book("atan10_11", "new/"),
	},
	{
		Name: "Русский язык (Рыбченкова)", Aliases: []string{"рус", "русс", "русский"}, Usage: "рус 50",
		argRe: reNum, Parse: parseImages, build: book("ribchenkova10-11", ""),
	},
	{
		Name: "Физика (Мякишев)", Aliases: []string{"физ", "физика", "мякишев"}, Usage: "физ 21-1 (параграф-задание)",
		argRe: reNumDash, Parse: parsePhysics, build: book("myakishev10", ""),
	},
	{
		Name: "Химия (Габриелян)", Aliases: []string{"хим", "химия"}, Usage: "хим 4-1 (параграф-вопрос)",
		argRe: reNumDash, Parse: parseChemistry, build: book("ostroumov10", ""),
	},
	{
		Name: "Литература (Лебедев)", Aliases: []string{"лит", "литра", "литература", "лебедев"}, Usage: "лит 1/134 (часть/страница)",
		argRe: rePartPage, Parse: parseLiterature,
		build: func(a string) string {
			m := rePartPage.FindStringSubmatch(a) // аргумент уже прошёл argRe, совпадение есть
			return reshebnik("lebedev_baz10", "part"+m[1]+"/"+m[2])
		},
	},
	{
		Name: "История (Мединский)", Aliases: []string{"ист", "история"}, Usage: "ист 1 (номер параграфа)",
		argRe: reNum, Parse: parseHistory, build: book("medinsky_vseobshaya10", ""),
	},
	{
		Name: "Английский (Forward)", Aliases: []string{"англ", "английский", "forward"}, Usage: "англ 8-9 (страницы)",
		argRe: reNumRange, Parse: parseEnglish,
		build: func(a string) string {
			return fmt.Sprintf("%s/otvet/otvet_txt.php?otvet1=/forward10/images/%s", reshakBase, a)
		},
	},
}

// Ищет предмет по первому слову команды (регистр и пробелы по краям не важны).
// Вызывается из resolveGDZ в gdz_send.go и из тестов.
func findGDZSubject(word string) (*gdzSubject, bool) {
	word = strings.ToLower(strings.TrimSpace(word))
	for i := range gdzSubjects {
		if slices.Contains(gdzSubjects[i].Aliases, word) {
			return &gdzSubjects[i], true
		}
	}
	return nil, false
}

// Проверяет аргумент регуляркой предмета и собирает URL страницы.
// Если аргумент не подходит — возвращает ошибку с примером из Usage.
func (s *gdzSubject) URL(arg string) (string, error) {
	if !s.argRe.MatchString(arg) {
		return "", fmt.Errorf("неверный номер, пример: /gdz %s", s.Usage)
	}
	return s.build(arg), nil
}

// Адрес страницы в общем скрипте reshebniki.php: predmet — код учебника на сайте,
// otvet — номер задания в формате этого учебника.
func reshebnik(predmet, otvet string) string {
	return fmt.Sprintf("%s/otvet/reshebniki.php?otvet=%s&predmet=%s", reshakBase, otvet, predmet)
}

// Возвращает функцию-сборщик для reshebnik: к номеру задания дописывает prefix
// (у геометрии это "new/") и подставляет код учебника predmet.
func book(predmet, prefix string) func(string) string {
	return func(a string) string { return reshebnik(predmet, prefix+a) }
}

// ───────────────────────────── ЗАГРУЗКА ─────────────────────────────

// Собирает URL (subject.URL), скачивает страницу через reshakGet (reshak.go)
// и отдаёт её Parse-функции предмета. В результат дописывает адрес страницы.
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
	base, err := url.Parse(pageURL) // нужен, чтобы превращать относительные пути картинок в абсолютные
	if err != nil {
		return GDZResult{}, err
	}
	res, err := s.Parse(doc, base)
	res.URL = pageURL
	return res, err
}

// ───────────────────────────── ПАРСЕРЫ ПРЕДМЕТОВ ─────────────────────────────
// Все парсеры стартуют с newResult, а заканчивают finish. Их вызывает FetchGDZ
// (через поле Parse) и тест на сохранённых страницах.

// Создаёт пустой результат и кладёт в него заголовок страницы (h1.titleh1).
func newResult(doc *goquery.Document) GDZResult {
	return GDZResult{Title: collapseSpaces(strings.TrimSpace(doc.Find("h1.titleh1").First().Text()))}
}

// Возвращает результат как есть, а если кусков нет — добавляет errGDZEmpty.
func finish(res GDZResult) (GDZResult, error) {
	if len(res.Parts) == 0 {
		return res, errGDZEmpty
	}
	return res, nil
}

// Добавляет в конец текстовый кусок: жирный заголовок и текст под ним.
// Пустой текст пропускает.
func (r *GDZResult) addBlock(heading, text string) {
	if text != "" {
		r.Parts = append(r.Parts, GDZPart{Text: "<b>" + heading + "</b>\n" + text})
	}
}

// Алгебра, геометрия, русский: на странице одни картинки решений.
func parseImages(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	res.Parts = pageImages(doc, base)
	return finish(res)
}

// Физика: картинки решений, а если на странице есть блок «ИИ-разбор» — то и его текст.
func parsePhysics(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	res.Parts = pageImages(doc, base)
	res.addBlock("ИИ-разбор", walkText(doc.Find(".ai-analysis__answer").First(), base))
	return finish(res)
}

// Химия: картинка решения и текст условия из .text_zad (без строк-шаблонов сайта,
// их вырезает dropBoilerplate).
func parseChemistry(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	res.Parts = pageImages(doc, base)
	cond := walkText(doc.Find("article.lcol .text_zad").First(), base)
	res.addBlock("Условие", dropBoilerplate(cond))
	return finish(res)
}

// Литература: десятки картинок решения, затем ссылки на презентации из
// fieldset.present. Блок .text_zad пропускается: на странице 134 в нём около
// 45 000 символов.
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

// История: весь ответ лежит в .mainInfo одним потоком — текст, таблицы и картинки
// вперемешку. Walker сохраняет этот порядок.
func parseHistory(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	res.Parts = mainInfoParts(doc, base)
	return finish(res)
}

// Английский (Forward): .mainInfo содержит только текст — оригинал и перевод
// курсивом. В заголовках решений убирается «#».
func parseEnglish(doc *goquery.Document, base *url.URL) (GDZResult, error) {
	res := newResult(doc)
	res.Parts = mainInfoParts(doc, base)
	for i := range res.Parts {
		res.Parts[i].Text = strings.ReplaceAll(res.Parts[i].Text, "Решение #", "Решение")
	}
	return finish(res)
}

// ───────────────────────────── ХЕЛПЕРЫ ─────────────────────────────

// Берёт все картинки решений в порядке документа: сначала div'ы pic_otvet1,
// pic_otvet1_dop1…N, затем pic_otvet2 и его dop. Повторы адресов отбрасывает.
// Каждый адрес берётся через imgURL.
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

// Достаёт из <img> настоящий адрес и делает его абсолютным относительно base.
// Ленивая подгрузка кладёт адрес в data-src (или data-original), обычные картинки —
// в src. Встроенные data:-картинки и не-http адреса пропускаются; пусто = адреса нет.
func imgURL(s *goquery.Selection, base *url.URL) string {
	for _, attr := range []string{"data-src", "data-original", "src"} {
		v := strings.TrimSpace(s.AttrOr(attr, ""))
		if v == "" || strings.HasPrefix(v, "data:") {
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

// Прогоняет через walker содержимое article.lcol .mainInfo — главного блока
// ответа на страницах истории и английского. Блока нет — вернёт пустой список.
func mainInfoParts(doc *goquery.Document, base *url.URL) []GDZPart {
	return walkParts(doc.Find("article.lcol .mainInfo").First(), base)
}

// Прогоняет через walker выбранный блок и возвращает его куски по порядку.
func walkParts(sel *goquery.Selection, base *url.URL) []GDZPart {
	w := newWalker(base)
	w.walk(sel)
	w.flush()
	return w.parts
}

// То же, что walkParts, но склеивает в одну строку только текстовые куски
// (картинки отбрасываются). Используется для блоков вроде .text_zad.
func walkText(sel *goquery.Selection, base *url.URL) string {
	var lines []string
	for _, p := range walkParts(sel, base) {
		if p.Text != "" {
			lines = append(lines, p.Text)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// Шаблонные первые слова строк, которые сайт добавляет к условию задачи.
var boilerplateRe = regexp.MustCompile(`^(Рассмотрим вариант решения|Приведем выдержку)`)

// Построчно выбрасывает из текста строки, начинающиеся с boilerplateRe
// (теги при проверке не учитываются; tagRe объявлен ниже). Вызывается из parseChemistry.
func dropBoilerplate(text string) string {
	var keep []string
	for line := range strings.SplitSeq(text, "\n") {
		if boilerplateRe.MatchString(tagRe.ReplaceAllString(line, "")) {
			continue
		}
		keep = append(keep, line)
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}

// Заменяет каждую серию пробельных символов (включая переносы и неразрывные
// пробелы) одним обычным пробелом. Края не обрезает: walker.text по ведущему
// пробелу решает, нужен ли разделитель между словами.
func collapseSpaces(s string) string {
	var sb strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				sb.WriteByte(' ')
			}
			prevSpace = true
			continue
		}
		prevSpace = false
		sb.WriteRune(r)
	}
	return sb.String()
}

// ───────────────────────────── HTML → Telegram-HTML ─────────────────────────────
//
// walker обходит DOM и собирает строки текста. Из всей разметки оставляет только
// <b> (b, strong, заголовки, .question) и <i> (i, em): их понимает parse_mode=HTML.
// Теги не переходят через границу строки: при сбросе строки (flush) открытые теги
// закрываются и открываются заново уже в следующей. Благодаря этому sendGDZ может
// резать текст по символу \n (splitText) и не ломать разметку.

var tagRe = regexp.MustCompile(`<[^>]+>`) // любой HTML-тег; для подсчёта видимого текста и для plainText

// Теги, которые начинают и заканчивают строку.
var blockTags = map[string]bool{
	"div": true, "p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "table": true, "tbody": true, "thead": true, "tr": true,
	"section": true, "article": true, "figure": true, "figcaption": true, "blockquote": true,
	"pre": true, "fieldset": true, "legend": true,
}

type walker struct {
	base    *url.URL
	parts   []GDZPart       // готовые куски; flush дописывает сюда строки
	line    strings.Builder // строка, которая собирается сейчас
	visible bool            // в текущей строке уже есть видимый текст
	lastSp  bool            // текущая строка заканчивается пробелом
	stack   []string        // открытые теги форматирования ("b", "i") в порядке открытия
	count   map[string]int  // сколько раз открыт каждый тег (вложенные <b><b> дают один тег)
	seenImg map[string]bool // адреса картинок, которые уже добавлены
	inCell  int             // больше 0, пока идёт обход ячейки таблицы; блоки внутри ячейки строку не рвут
	cellIdx int             // номер ячейки в текущей строке таблицы; со второй ячейки ставится « | »
	gap     bool            // перед следующей строкой нужна пустая строка (после заголовка или вопроса)
}

func newWalker(base *url.URL) *walker {
	return &walker{base: base, count: map[string]int{}, seenImg: map[string]bool{}}
}

// Обходит дочерние узлы выбранного элемента по порядку (см. node).
func (w *walker) walk(s *goquery.Selection) {
	s.Contents().Each(func(_ int, n *goquery.Selection) { w.node(n) })
}

// Обрабатывает один узел. Текст кладёт в текущую строку; <br> и блочные теги
// заканчивают строку; <img> становится отдельным куском-картинкой (повторы
// пропускаются); b, strong, em, i, заголовки и .question включают форматирование;
// ячейки таблицы разделяются « | ». Остальные теги обходятся без разметки.
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
			if !w.seenImg[u] {
				w.seenImg[u] = true
				w.parts = append(w.parts, GDZPart{Image: u})
			}
		}
		return
	}

	isHead := len(name) == 2 && name[0] == 'h' && name[1] >= '1' && name[1] <= '6'
	isQuestion := n.HasClass("question")
	fmtTag := ""
	switch {
	case name == "b" || name == "strong" || isHead || isQuestion:
		fmtTag = "b"
	case name == "i" || name == "em":
		fmtTag = "i"
	}

	block := blockTags[name] && w.inCell == 0
	if block {
		w.flush()
	}
	if isHead || isQuestion {
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

// Решает, пропускать ли узел вместе с содержимым: скрипты, стили, формы, кнопки,
// навигация, блоки рекламы (id yandex_rtb…, классы empty_place, readmore-js-toggle,
// text-zad-note) и всё с inline-стилем display:none. Вызывается из node.
func skipNode(n *goquery.Selection, name string) bool {
	switch name {
	case "script", "style", "noscript", "noindex", "iframe", "button", "form", "nav", "svg", "audio", "video":
		return true
	}
	if id, _ := n.Attr("id"); strings.HasPrefix(id, "yandex_rtb") {
		return true
	}
	if slices.ContainsFunc([]string{"empty_place", "readmore-js-toggle", "text-zad-note"}, n.HasClass) {
		return true
	}
	if st, _ := n.Attr("style"); strings.Contains(strings.ReplaceAll(st, " ", ""), "display:none") {
		return true
	}
	return false
}

// Добавляет текстовый узел в текущую строку: схлопывает пробелы (collapseSpaces),
// экранирует спецсимволы HTML и отбрасывает ведущий пробел в начале строки и после
// другого пробела.
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

// Дописывает в sb открывающий тег <tag>. Работает с walker.line и с builder'ом
// закрывающих тегов в flush.
func writeOpen(sb *strings.Builder, tag string) {
	sb.WriteByte('<')
	sb.WriteString(tag)
	sb.WriteByte('>')
}

// Дописывает в sb закрывающий тег </tag>.
func writeClose(sb *strings.Builder, tag string) {
	sb.WriteString("</")
	sb.WriteString(tag)
	sb.WriteByte('>')
}

// Открывает тег форматирования в текущей строке. Если тег уже открыт (вложенный
// <b> внутри <b>), только увеличивает счётчик и второй раз не пишет.
func (w *walker) open(tag string) {
	if w.count[tag] == 0 {
		writeOpen(&w.line, tag)
		w.stack = append(w.stack, tag)
	}
	w.count[tag]++
}

// Закрывает тег форматирования, когда закрыт самый внешний из вложенных. Теги,
// открытые после него, закрываются и тут же открываются заново, чтобы разметка
// осталась правильно вложенной.
func (w *walker) close(tag string) {
	w.count[tag]--
	if w.count[tag] > 0 {
		return
	}
	idx := slices.Index(w.stack, tag) // тег лежит в стеке не больше одного раза (см. open)
	if idx < 0 {
		return
	}
	tail := slices.Clone(w.stack[idx+1:]) // теги, открытые после закрываемого
	for _, t := range slices.Backward(w.stack[idx:]) {
		writeClose(&w.line, t)
	}
	for _, t := range tail {
		writeOpen(&w.line, t)
	}
	w.stack = append(w.stack[:idx], tail...)
}

// Завершает текущую строку: закрывает открытые теги и открывает их заново в
// начале новой строки. Строку без видимого текста выбрасывает. Готовую строку
// дописывает к предыдущему текстовому куску через \n (после заголовка или вопроса —
// через пустую строку), а после картинки начинает новый кусок.
func (w *walker) flush() {
	line := w.line.String()
	w.line.Reset()
	w.visible, w.lastSp = false, false

	var closing strings.Builder
	for _, t := range slices.Backward(w.stack) {
		writeClose(&closing, t)
	}
	for _, t := range w.stack {
		writeOpen(&w.line, t)
	}

	line = strings.TrimSpace(line + closing.String())
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
