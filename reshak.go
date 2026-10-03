package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	reshakBase = "https://reshak.ru"
	browserUA  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
)

// Cookie jar нужен, чтобы сохранялись __ddg* куки между запросами.
var reshakClient = func() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 20 * time.Second}
}()

func reshakGet(ctx context.Context, rawURL, accept, referer string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := reshakClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: статус %d", rawURL, resp.StatusCode)
	}
	return resp, nil
}

type SolutionImage struct {
	Solution int    // 1 или 2
	URL      string // полный URL картинки
}

// FetchSolutionImages возвращает URL картинок решений в порядке: №1, его доп., №2, его доп.
func FetchSolutionImages(ctx context.Context, exercise int) ([]SolutionImage, string, error) {
	pageURL := fmt.Sprintf("%s/otvet/otvet15.php?otvet=%d", reshakBase, exercise)
	resp, err := reshakGet(ctx, pageURL, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "")
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, "", err
	}
	base, _ := url.Parse(pageURL)

	groups := []struct {
		solution int
		selector string
	}{
		{1, ".pic_otvet1 img"},
		{1, ".pic_otvet1_dop1 img"},
		{2, ".pic_otvet2 img"},
		{2, ".pic_otvet2_dop1 img"},
	}

	var out []SolutionImage
	seen := map[string]bool{}
	for _, g := range groups {
		doc.Find(g.selector).Each(func(_ int, s *goquery.Selection) {
			src, _ := s.Attr("data-src") // lazyload
			if src == "" {
				src, _ = s.Attr("src")
			}
			src = strings.TrimSpace(src)
			if src == "" {
				return
			}
			ref, err := url.Parse(src)
			if err != nil {
				return
			}
			full := base.ResolveReference(ref).String()
			if seen[full] {
				return
			}
			seen[full] = true
			out = append(out, SolutionImage{Solution: g.solution, URL: full})
		})
	}
	if len(out) == 0 {
		return nil, "", fmt.Errorf("картинки решения не найдены")
	}

	// Условие задачи (необязательно)
	task := strings.TrimSpace(doc.Find(".text_zad").First().Text())
	return out, task, nil
}

func DownloadImage(ctx context.Context, imgURL, referer string) ([]byte, error) {
	resp, err := reshakGet(ctx, imgURL, "image/avif,image/webp,image/png,image/*;q=0.8,*/*;q=0.5", referer)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // максимум 10 МБ
}