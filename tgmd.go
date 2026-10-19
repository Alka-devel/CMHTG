package main

// tgmd.go — превращает текст сообщения Telegram и его entities в строку MarkdownV2.
//
// Зачем: оповещения хранятся в .md, а при рассылке уходят обратно с parse_mode=MarkdownV2.
// Telegram сам разбирает свой MarkdownV2, так что своего парсера не нужно: форматирование
// и кастомные эмодзи (![😇](tg://emoji?id=...)) возвращаются в точности как были.
// Обратной функции нет намеренно.

import (
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/mymmrac/telego"
)

type mdEnt struct {
	typ        string
	start, end int // в единицах UTF-16, end не включается
	url        string
	userID     int64
	emojiID    string
	lang       string
}

func utf16Len(s string) int { return len(utf16.Encode([]rune(s))) }

// Меньше rank — «внешнее»: если у нескольких сущностей совпадает диапазон,
// цитата открывается раньше форматирования, форматирование раньше ссылки.
func mdRank(t string) int {
	switch t {
	case telego.EntityTypeBlockquote, telego.EntityTypeExpandableBlockquote:
		return 0
	case telego.EntityTypeBold, telego.EntityTypeUnderline, telego.EntityTypeItalic,
		telego.EntityTypeStrikethrough, telego.EntityTypeSpoiler:
		return 1
	case telego.EntityTypeTextLink, telego.EntityTypeTextMention:
		return 2
	}
	return 3
}

// Порядок внутри rank 1 при полном совпадении диапазонов. Подчёркнутый снаружи,
// курсив внутри: так «___» читается парсером Telegram однозначно.
func mdOrder(t string) int {
	return slices.Index([]string{
		telego.EntityTypeBold, telego.EntityTypeUnderline, telego.EntityTypeItalic,
		telego.EntityTypeStrikethrough, telego.EntityTypeSpoiler,
	}, t)
}

func isCodeEnt(t string) bool {
	return t == telego.EntityTypeCode || t == telego.EntityTypePre
}

// Оставляет только то, что выражается в MarkdownV2, склеивает пересекающиеся
// сущности одного типа и выкидывает форматирование внутри code/pre.
func mdPrepare(u []uint16, ents []telego.MessageEntity) []mdEnt {
	byType := map[string][]mdEnt{}
	var all []mdEnt
	for _, e := range ents {
		m := mdEnt{
			typ: e.Type, start: e.Offset, end: e.Offset + e.Length,
			url: e.URL, emojiID: e.CustomEmojiID, lang: e.Language,
		}
		if e.User != nil {
			m.userID = e.User.ID
		}
		if m.start < 0 || m.end > len(u) || m.start >= m.end {
			continue
		}
		switch m.typ {
		case telego.EntityTypeBold, telego.EntityTypeItalic, telego.EntityTypeUnderline,
			telego.EntityTypeStrikethrough, telego.EntityTypeSpoiler:
			if strings.TrimSpace(string(utf16.Decode(u[m.start:m.end]))) == "" {
				continue // форматирование одних пробелов Telegram не любит
			}
			byType[m.typ] = append(byType[m.typ], m)
		case telego.EntityTypeTextLink:
			if m.url != "" {
				all = append(all, m)
			}
		case telego.EntityTypeTextMention:
			if m.userID != 0 {
				all = append(all, m)
			}
		case telego.EntityTypeCustomEmoji:
			if m.emojiID != "" {
				all = append(all, m)
			}
		case telego.EntityTypeCode, telego.EntityTypePre,
			telego.EntityTypeBlockquote, telego.EntityTypeExpandableBlockquote:
			all = append(all, m)
		}
	}
	for _, list := range byType {
		slices.SortFunc(list, func(a, b mdEnt) int { return a.start - b.start })
		cur := list[0]
		for _, e := range list[1:] {
			if e.start <= cur.end {
				cur.end = max(cur.end, e.end)
				continue
			}
			all = append(all, cur)
			cur = e
		}
		all = append(all, cur)
	}

	var res []mdEnt
	for i, e := range all {
		nested := false
		for j, c := range all {
			if i != j && isCodeEnt(c.typ) && e.start >= c.start && e.start < c.end {
				nested = true
				break
			}
		}
		if !nested {
			res = append(res, e)
		}
	}
	slices.SortFunc(res, func(a, b mdEnt) int {
		if a.start != b.start {
			return a.start - b.start
		}
		if a.end != b.end {
			return b.end - a.end
		}
		if ra, rb := mdRank(a.typ), mdRank(b.typ); ra != rb {
			return ra - rb
		}
		return mdOrder(a.typ) - mdOrder(b.typ)
	})
	return res
}

func mdOpen(e mdEnt) string {
	switch e.typ {
	case telego.EntityTypeBold:
		return "*"
	case telego.EntityTypeItalic:
		return "_"
	case telego.EntityTypeUnderline:
		return "__"
	case telego.EntityTypeStrikethrough:
		return "~"
	case telego.EntityTypeSpoiler:
		return "||"
	case telego.EntityTypeCode:
		return "`"
	case telego.EntityTypePre:
		return "```" + e.lang + "\n"
	case telego.EntityTypeTextLink, telego.EntityTypeTextMention:
		return "["
	case telego.EntityTypeCustomEmoji:
		return "!["
	case telego.EntityTypeBlockquote:
		return ">"
	case telego.EntityTypeExpandableBlockquote:
		return "**>"
	}
	return ""
}

func mdClose(e mdEnt) string {
	switch e.typ {
	case telego.EntityTypeBold:
		return "*"
	case telego.EntityTypeItalic:
		return "_"
	case telego.EntityTypeUnderline:
		return "__"
	case telego.EntityTypeStrikethrough:
		return "~"
	case telego.EntityTypeSpoiler:
		return "||"
	case telego.EntityTypeCode:
		return "`"
	case telego.EntityTypePre:
		return "\n```"
	case telego.EntityTypeTextLink:
		return "](" + mdURL(e.url) + ")"
	case telego.EntityTypeTextMention:
		return "](tg://user?id=" + itoa64(e.userID) + ")"
	case telego.EntityTypeCustomEmoji:
		return "](tg://emoji?id=" + e.emojiID + ")"
	case telego.EntityTypeExpandableBlockquote:
		return "||"
	}
	return ""
}

func itoa64(n int64) string {
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			break
		}
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// Внутри (...) ссылки экранируются только ) и \.
func mdURL(s string) string {
	return strings.NewReplacer(`\`, `\\`, `)`, `\)`).Replace(s)
}

const mdSpecial = "_*[]()~`>#+-=|{}.!\\"

// Внутри code/pre экранируются только ` и \, везде остальное — весь mdSpecial.
func mdWrite(sb *strings.Builder, r rune, code bool) {
	switch {
	case code && (r == '`' || r == '\\'), !code && strings.ContainsRune(mdSpecial, r):
		sb.WriteByte('\\')
	}
	sb.WriteRune(r)
}

// entitiesToMarkdownV2 собирает MarkdownV2 из текста и entities исходного сообщения.
// Незакрытые «перехлёсты» (жирный 0–3, курсив 2–5) раскладываются корректно:
// сущность закрывается и тут же открывается заново.
func entitiesToMarkdownV2(text string, ents []telego.MessageEntity) string {
	u := utf16.Encode([]rune(text))
	es := mdPrepare(u, ents)

	var sb strings.Builder
	var stack []mdEnt
	next, lineStart := 0, true
	for i := 0; ; {
		// закрываем всё, что закончилось; то, что лежало выше закрытого, переоткрываем
		for {
			k := slices.IndexFunc(stack, func(e mdEnt) bool { return e.end <= i })
			if k < 0 {
				break
			}
			for j := len(stack) - 1; j >= k; j-- {
				sb.WriteString(mdClose(stack[j]))
			}
			var keep []mdEnt
			for _, e := range stack[k+1:] {
				if e.end > i {
					keep = append(keep, e)
				}
			}
			stack = stack[:k]
			for _, e := range keep {
				sb.WriteString(mdOpen(e))
				stack = append(stack, e)
			}
		}
		for next < len(es) && es[next].start <= i {
			e := es[next]
			next++
			if mdRank(e.typ) == 0 && !lineStart {
				continue // цитата не с начала строки в MarkdownV2 не выражается
			}
			sb.WriteString(mdOpen(e))
			stack = append(stack, e)
		}
		if i >= len(u) {
			break
		}

		r, w := rune(u[i]), 1
		if utf16.IsSurrogate(r) && i+1 < len(u) {
			r, w = utf16.DecodeRune(r, rune(u[i+1])), 2
		}
		code := len(stack) > 0 && isCodeEnt(stack[len(stack)-1].typ)
		inQuote := slices.ContainsFunc(stack, func(e mdEnt) bool { return mdRank(e.typ) == 0 })
		if r == '\n' && inQuote && !code {
			sb.WriteString("\n>")
		} else {
			mdWrite(&sb, r, code)
		}
		lineStart = r == '\n'
		i += w
	}
	return sb.String()
}
