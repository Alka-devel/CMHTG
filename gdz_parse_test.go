package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// Тест на сохранённых страницах. Запуск:
//
//	GDZ_SAMPLES=/путь/до/репозитория/sites go test -run TestGDZParseSamples -v
func TestGDZParseSamples(t *testing.T) {
	dir := os.Getenv("GDZ_SAMPLES")
	if dir == "" {
		t.Skip("задайте GDZ_SAMPLES — папка с сохранёнными html")
	}
	cases := []struct {
		key      string // подстрока в имени файла
		alias    string
		wantImgs int
		wantText bool
	}{
		{"Алимов", "алг", 2, false},
		{"Атанасян", "геом", 2, false},
		{"Рыбченкова", "рус", 3, false},
		{"Мякишев", "физ", 2, true},
		{"Габриелян", "хим", 1, true},
		{"Лебедев", "лит", 35, true}, // текст = ссылка на презентацию
		{"Мединский", "ист", 6, true},
		{"Forward", "англ", 0, true},
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.html"))
	for _, c := range cases {
		var file string
		for _, f := range files {
			if strings.Contains(filepath.Base(f), c.key) {
				file = f
			}
		}
		if file == "" {
			t.Errorf("%s: файл не найден", c.key)
			continue
		}
		fh, _ := os.Open(file)
		doc, err := goquery.NewDocumentFromReader(fh)
		fh.Close()
		if err != nil {
			t.Fatal(err)
		}
		canon, _ := doc.Find("link[rel=canonical]").Attr("href")
		base, _ := url.Parse(canon)
		subj, ok := findGDZSubject(c.alias)
		if !ok {
			t.Fatalf("нет предмета %q", c.alias)
		}
		res, err := subj.Parse(doc, base)
		if err != nil {
			t.Errorf("%s: %v", c.key, err)
			continue
		}
		imgs, texts, chars := 0, 0, 0
		for _, p := range res.Parts {
			if p.Image != "" {
				imgs++
			} else {
				texts++
				chars += len([]rune(p.Text))
			}
		}
		t.Logf("%-11s title=%q imgs=%d textParts=%d chars=%d", c.key, res.Title, imgs, texts, chars)
		if imgs != c.wantImgs {
			t.Errorf("%s: картинок %d, ждали %d", c.key, imgs, c.wantImgs)
		}
		if (chars > 0) != c.wantText {
			t.Errorf("%s: текст есть=%v, ждали %v", c.key, chars > 0, c.wantText)
		}
		if strings.Contains(strings.Join(partsText(res), "\n"), "yandex_rtb") || strings.Contains(strings.Join(partsText(res), "\n"), "Ya.Context") {
			t.Errorf("%s: в тексте остался рекламный код", c.key)
		}
		if os.Getenv("GDZ_DUMP") == "1" {
			for _, p := range res.Parts {
				if p.Image != "" {
					t.Logf("   [IMG] %s", p.Image)
				} else {
					t.Logf("   [TXT]\n%s", p.Text)
				}
			}
		}
	}
}

func partsText(r GDZResult) []string {
	var out []string
	for _, p := range r.Parts {
		out = append(out, p.Text)
	}
	return out
}

func TestGDZURLs(t *testing.T) {
	ok := map[string]string{
		"алг 74":    "https://reshak.ru/otvet/otvet15.php?otvet=74",
		"геом 222":  "https://reshak.ru/otvet/reshebniki.php?otvet=new/222&predmet=atan10_11",
		"рус 50":    "https://reshak.ru/otvet/reshebniki.php?otvet=50&predmet=ribchenkova10-11",
		"физ 21-1":  "https://reshak.ru/otvet/reshebniki.php?otvet=21-1&predmet=myakishev10",
		"хим 4-1":   "https://reshak.ru/otvet/reshebniki.php?otvet=4-1&predmet=ostroumov10",
		"лит 1/134": "https://reshak.ru/otvet/reshebniki.php?otvet=part1/134&predmet=lebedev_baz10",
		"лит 1 134": "https://reshak.ru/otvet/reshebniki.php?otvet=part1/134&predmet=lebedev_baz10",
		"ист 1":     "https://reshak.ru/otvet/reshebniki.php?otvet=1&predmet=medinsky_vseobshaya10",
		"англ 8-9":  "https://reshak.ru/otvet/otvet_txt.php?otvet1=/forward10/images/8-9",
	}
	for in, want := range ok {
		f := strings.SplitN(in, " ", 2)
		s, _ := findGDZSubject(f[0])
		got, err := s.URL(f[1])
		if err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"алг 74&predmet=x", "алг ../1", "физ 21", "лит 3/1", "англ a-b", "алг "} {
		f := strings.SplitN(in, " ", 2)
		s, _ := findGDZSubject(f[0])
		if _, err := s.URL(f[1]); err == nil {
			t.Errorf("%q должен быть отклонён", in)
		}
	}
}
