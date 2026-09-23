package main

import (
	"testing"
	"time"

	"github.com/fogleman/gg"
)

// TestRenderSchedulePreview — не настоящий unit-тест, а быстрый способ
// глазами посмотреть картинку. Запуск:
//
//	go test -run TestRenderSchedulePreview -v
//
// Результат появится в файле preview.png в корне проекта.
func TestRenderSchedulePreview(t *testing.T) {
	day, _ := week.GetDay(time.Now())

	// currentIndex = 2 → подсвечивается 3-й урок (индексация с нуля), как в макете
	// scale = DefaultScale (3x) — рендерим в повышенном разрешении, чтобы текст не был мыльным
	img, err := RenderScheduleImage(day, 2, "18 мин", DefaultScale, Empty)
	if err != nil {
		t.Fatalf("рендер не удался: %v", err)
	}

	if err := gg.SavePNG("preview.png", img); err != nil {
		t.Fatalf("сохранение png не удалось: %v", err)
	}

	t.Log("сохранено в preview.png")
}
