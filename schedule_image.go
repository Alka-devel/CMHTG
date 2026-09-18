package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

var (
	regularFont *opentype.Font
	mediumFont  *opentype.Font
)

//go:embed assets/Roboto-Regular.ttf
var robotoRegularBytes []byte

//go:embed assets/Roboto-Medium.ttf
var robotoMediumBytes []byte

func init() {
	var err error
	regularFont, err = opentype.Parse(robotoRegularBytes)
	if err != nil {
		regularFont = nil
	}
	mediumFont, err = opentype.Parse(robotoMediumBytes)
	if err != nil {
		mediumFont = nil
	}
}

func setFont(dc *gg.Context, f *opentype.Font, sizePx float64) error {
	if f == nil {
		return fmt.Errorf("шрифт не загружен")
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: sizePx, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return err
	}
	dc.SetFontFace(face)
	return nil
}

var (
	colBg        = mustHex("#F1EFE8") // фон карточки (surface-1)
	colRowBg     = mustHex("#FFFFFF") // фон обычной строки (surface-2)
	colBorder    = mustHex("#E3E1D9") // разделитель
	colTextMain  = mustHex("#1C1C1A") // основной текст
	colTextMuted = mustHex("#5F5E5A") // вторичный текст (номер, кабинет, время)

	// подсветка текущего урока — teal-тональный контейнер
	colHighlightBg   = mustHex("#dff5ec") // teal 50 c прозрачностью ~под фон
	colHighlightText = mustHex("#085041") // teal 800 — текст на подсветке
	colGroupText     = mustHex("#b8e9d5")
)

func mustHex(h string) color.RGBA {
	var r, g, b int
	fmt.Sscanf(h, "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}

const (
	baseImgW       = 360.0
	basePad        = 20.0
	baseMiniPad    = 5.0
	baseRowH       = 40.0
	baseRowGap     = 4.0
	baseRowRadius  = 10.0
	baseCardRadius = 16.0
	baseRowPadX    = 14.0 // внутренний отступ строки слева/справа для текста
	baseSubjectX   = 34.0 // отступ начала текста предмета от левого края строки
	baseMinTextGap = 8.0  // минимальный зазор между концом названия предмета и текстом справа

	baseHeaderTop  = 52.0 // y, с которого начинаются строки уроков (под шапкой с датой)
	baseFooterGap  = 8.0  // зазор от последней строки до разделителя подвала
	baseFooterText = 32.0 // высота текстовой части подвала после разделителя
	baseBottomPad  = 8.0  // отступ снизу под последней строкой подвала

	baseFontHeader = 15.0
	baseFontNum    = 12.0
	baseFontTitle  = 13.0
	baseFontSmall  = 12.0
	DefaultScale   = 3.0
)

func fitText(dc *gg.Context, text string, maxWidth float64) string {
	if maxWidth <= 0 {
		return text
	}
	if w, _ := dc.MeasureString(text); w <= maxWidth {
		return text
	}
	runes := []rune(text)
	for i := len(runes) - 1; i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if w, _ := dc.MeasureString(candidate); w <= maxWidth {
			return candidate
		}
	}
	return "…"
}

func RenderScheduleImage(day ScheduleDay, currentIndex int, timeLeft string, scale float64) (image.Image, error) {
	if scale <= 0 {
		scale = DefaultScale
	}
	s := func(v float64) float64 { return v * scale }
	var rowsCount int
	for _, entry := range day.Entries {
		l := strings.Split(entry.Subject, "/")
		if len(l) > 1 {
			for i, str := range l {
				l[i] = strings.TrimSpace(str)
				rowsCount++
			}
			continue
		}
		rowsCount++
	}

	imgW := s(baseImgW)     // ширина холста в реальных px
	pad := s(basePad)       // общий отступ карточки от краёв (слева/справа/сверху и т.п.)
	rowH := s(baseRowH)     // высота одной строки-урока
	rowGap := s(baseRowGap) // вертикальный зазор между соседними строками

	contentBottom := baseHeaderTop
	if rowsCount > 0 {
		contentBottom += float64(rowsCount)*(baseRowH+baseRowGap) - baseRowGap
	}
	baseImgH := contentBottom + baseFooterGap + 1 /*линия*/ + baseFooterText + baseBottomPad
	imgH := s(baseImgH) // переводим в реальные px с учётом scale

	dc := gg.NewContext(int(imgW), int(imgH))

	dc.SetColor(colBg)
	dc.DrawRoundedRectangle(0, 0, imgW, imgH, s(baseCardRadius))
	dc.Fill()

	if err := setFont(dc, mediumFont, s(baseFontHeader)); err != nil {
		return nil, fmt.Errorf("шрифт (medium): %w", err)
	}
	dc.SetColor(colTextMain)
	header := formatHeaderDate(day.Date)
	dc.DrawString(header, pad, s(28))

	dc.SetColor(colBorder)
	dc.SetLineWidth(s(1))
	dc.DrawLine(pad, s(40), imgW-pad, s(40))
	dc.Stroke()

	y := s(baseHeaderTop)
	for _, entry := range day.Entries {
		if entry.IsSE == true {
			fmt.Printf("\n\n\nSE\n\n\n")
		}
		isCurrent := entry.Number == currentIndex
		var groupPad float64 = pad
		if err := setFont(dc, mediumFont, s(baseFontNum)); err != nil {
			return nil, err
		}
		rd, _ := dc.MeasureString(fmt.Sprintf("%d", entry.Number))
		if entry.Addi == true {
			groupPad = pad + s(baseRowPadX)*2 + rd
		}
		bg := colRowBg
		if isCurrent {
			bg = colHighlightBg
		}
		dc.SetColor(bg)
		dc.DrawRoundedRectangle(groupPad, y, imgW-pad-groupPad, rowH, s(baseRowRadius))
		dc.Fill()

		textColor := colTextMuted
		mainColor := colTextMain
		if isCurrent {
			textColor = colHighlightText
			mainColor = colHighlightText
		}

		cy := y + rowH/2 + s(4)
		if err := setFont(dc, mediumFont, s(baseFontNum)); err != nil {
			return nil, err
		}
		numW, _ := dc.MeasureString(fmt.Sprint(entry.Number))
		dc.SetColor(textColor)
		dc.DrawString(fmt.Sprintf("%d", entry.Number), pad+s(baseRowPadX), cy)
		right := fmt.Sprintf("каб. %d", entry.Room)
		groupT := func() string {
			if entry.IsIT == true {
				return "И-Т"
			} else if entry.IsSE == true {
				return "С-Э"
			}
			return ""
		}
		if isCurrent {
			right = "сейчас"
		}
		if err := setFont(dc, regularFont, s(baseFontSmall)); err != nil {
			return nil, err
		}
		groupTeW, groupTeH := dc.MeasureString(groupT())
		if groupTeW > 2 {
			groupTeW += s(baseMiniPad) * 3
		}
		rw, _ := dc.MeasureString(right)
		rightX := imgW - pad - s(baseRowPadX) - rw
		rightTeX := imgW - pad - s(baseRowPadX) - rw - groupTeW
		subjectX := groupPad + s(baseSubjectX)
		if entry.Addi == true {
			subjectX -= s(baseSubjectX)/2 + numW/2
		}
		maxSubjectWidth := rightX - s(baseMinTextGap) - subjectX - groupTeW
		if err := setFont(dc, mediumFont, s(baseFontTitle)); err != nil {
			return nil, err
		}
		dc.SetColor(mainColor)
		subject := fitText(dc, entry.Subject, maxSubjectWidth)
		dc.DrawString(subject, subjectX, cy)
		if err := setFont(dc, regularFont, s(baseFontSmall)); err != nil {
			return nil, err
		}
		dc.SetColor(textColor)
		dc.DrawString(right, rightX, cy)
		if entry.IsIT == true {
			dc.SetColor(colGroupText)
			dc.DrawRoundedRectangle(rightTeX, y+((rowH-groupTeH)/2-2), groupTeW-s(baseMiniPad), groupTeH+8, groupTeH/2+4)
			dc.Fill()
			dc.SetColor(textColor)
			dc.DrawString(groupT(), rightTeX+s(baseMiniPad), cy)
		}
		if entry.IsSE == true {
			dc.SetColor(colGroupText)
			dc.DrawRoundedRectangle(rightTeX, y+((rowH-groupTeH)/2-2), groupTeW-s(baseMiniPad), groupTeH+8, groupTeH/2+4)
			dc.Fill()
			dc.SetColor(textColor)
			dc.DrawString(groupT(), rightTeX+s(baseMiniPad), cy)
		}
		y += rowH + rowGap
	}

	dc.SetColor(colBorder)
	dc.DrawLine(pad, y+s(baseFooterGap), imgW-pad, y+s(baseFooterGap))
	dc.Stroke()

	if err := setFont(dc, regularFont, s(baseFontSmall)); err != nil {
		return nil, err
	}
	dc.SetColor(colTextMuted)
	dc.DrawString("До конца", pad, y+s(baseFooterText))

	if err := setFont(dc, mediumFont, s(baseFontSmall)); err != nil {
		return nil, err
	}
	dc.SetColor(colTextMain)
	w, _ := dc.MeasureString(timeLeft)
	dc.DrawString(timeLeft, imgW-pad-w, y+s(baseFooterText))

	return dc.Image(), nil
}

func formatHeaderDate(d time.Time) string {
	names := map[time.Month]string{
		time.January: "января", time.February: "февраля", time.March: "марта",
		time.April: "апреля", time.May: "мая", time.June: "июня",
		time.July: "июля", time.August: "августа", time.September: "сентября",
		time.October: "октября", time.November: "ноября", time.December: "декабря",
	}
	weekdays := map[time.Weekday]string{
		time.Monday:    "Понедельник",
		time.Tuesday:   "Вторник",
		time.Wednesday: "Среда",
		time.Thursday:  "Четверг",
		time.Friday:    "Пятница",
		time.Saturday:  "Суббота",
		time.Sunday:    "Воскресенье",
	}
	return fmt.Sprintf("%s, %d %s", weekdays[d.Weekday()], d.Day(), names[d.Month()])
}

func EncodePNG(img image.Image) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := gg.NewContextForImage(img).EncodePNG(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
