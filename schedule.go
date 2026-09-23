package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type Period struct {
	Start time.Duration
	End   time.Duration
}

func P(startH, startM, endH, endM int) Period {
	return Period{
		Start: time.Duration(startH)*time.Hour + time.Duration(startM)*time.Minute,
		End:   time.Duration(endH)*time.Hour + time.Duration(endM)*time.Minute,
	}
}

type LessonStatus struct {
	IsLesson    bool
	Finished    bool
	LessonIndex int
	TimeLeft    time.Duration
	NextBreak   time.Duration
}

func (s LessonStatus) String() string {
	if s.Finished {
		return "УРОКОВ НЕЕТ 😝😝🤟🤘"
	}
	if s.IsLesson {
		return fmt.Sprintf(
			"идёт урок №%d, до конца %s, после него перемена %s",
			s.LessonIndex+1, fmtDur(s.TimeLeft), fmtDur(s.NextBreak),
		)
	}
	return fmt.Sprintf(
		"идёт перемена, до урока №%d осталось %s (длительность перемены: %s)",
		s.LessonIndex+1, fmtDur(s.TimeLeft), fmtDur(s.NextBreak),
	)
}

func Status(now time.Time) LessonStatus {
	lessons := scheduleFor(now)
	n := timeOfDay(now)
	u, ok := week.GetDay(now)
	if !ok {
		fmt.Println("ne ok")
		return LessonStatus{}
	}
	maxNumber := 0
	for _, e := range u.Entries {
		if e.Number > maxNumber {
			maxNumber = e.Number
		}
	}
	if maxNumber > 0 && maxNumber < len(lessons) {
		lessons = lessons[:maxNumber]
	}

	for i, l := range lessons {
		switch {
		case (n >= l.Start && n < l.End):
			var next time.Duration
			if i+1 < len(lessons) {
				next = lessons[i+1].Start - l.End
			}
			return LessonStatus{
				IsLesson:    true,
				LessonIndex: i,
				TimeLeft:    l.End - n,
				NextBreak:   next,
			}
		case n < l.Start:
			left := l.Start - n
			return LessonStatus{
				IsLesson:    false,
				LessonIndex: i,
				TimeLeft:    left,
				NextBreak:   left,
			}
		}
	}
	return LessonStatus{Finished: true, LessonIndex: -1}
}
func (s LessonStatus) Params(t telego.ChatID, day ScheduleDay) *telego.SendMessageParams {
	if time.Now().Weekday() == time.Sunday || (time.Now().Weekday() == time.Saturday && s.Finished) {
		return tu.MessageWithEntities(
			t,
			tu.Entity("Отдыхай ёпта"),
			tu.Entity("😘").CustomEmoji("5381841785666413682"),
		)
	}
	if s.Finished {
		return tu.MessageWithEntities(
			t,
			tu.Entity("🍱").CustomEmoji("5332596498104340386"),
			tu.Entity("🫔").CustomEmoji("5332497365964182852"),
			tu.Entity("🍚").CustomEmoji("5332436665191389001"),
			tu.Entity("\n"),
			tu.Entity("УРОКОВ НЕЕТ"),
			tu.Entity("😝").CustomEmoji("5370564490037303348"),
			tu.Entity("😝").CustomEmoji("5348397248994617112"),
		)
	}
	entry, ok := day.FindEntry(s.LessonIndex + 1)
	var subject string = "не у нас"
	var room int
	if ok {
		subject, room = entry.Subject, entry.Room
	}
	if s.IsLesson {
		if s.NextBreak == 0 {
			return tu.MessageWithEntities(
				t,
				tu.Entity(fmt.Sprintf("Урок: %s, %d\n", strings.TrimSpace(subject), room)),
				tu.Entity(fmt.Sprintf("До конца: %s\n", fmtDur(s.TimeLeft))),
				tu.Entity("ЙОО ЭТО ПОСЛЕДНИЙ УРООК!!"),
				tu.Entity("😘").CustomEmoji("5381841785666413682"),
				tu.Entity("\nА ПОТОМ ДОМОООЙ"),
				tu.Entity("😝").CustomEmoji("5370564490037303348"),
				tu.Entity("😝").CustomEmoji("5348397248994617112"),
			)
		} else {
			return tu.MessageWithEntities(
				t,
				tu.Entity(fmt.Sprintf("Урок: %s, %d\n", strings.TrimSpace(subject), room)),
				tu.Entity(fmt.Sprintf("До конца: %s\n", fmtDur(s.TimeLeft))),
				tu.Entity(fmt.Sprintf("После него перемена: %s", fmtDur(s.NextBreak))),
			)
		}
	}
	if s.NextBreak == s.TimeLeft {
		fmt.Println(s.NextBreak, s.TimeLeft)
		return tu.MessageWithEntities(
			t,
			tu.Entity("Перемена\n"),
			tu.Entity(fmt.Sprintf("До урока: %s\n", fmtDur(s.TimeLeft))),
		)
	}
	return tu.MessageWithEntities(
		t,
		tu.Entity("Перемена\n"),
		tu.Entity(fmt.Sprintf("До урока: %s\n", fmtDur(s.TimeLeft))),
		tu.Entity(fmt.Sprintf("После него перемена: %s", fmtDur(s.NextBreak))),
		tu.Entity("😘").CustomEmoji("5381841785666413682"),
	)
}

func (d ScheduleDay) FindEntry(number int) (LessonEntry, bool) {
	for _, e := range d.Entries {
		if e.Number == number {
			return e, true
		}
	}
	return LessonEntry{}, false
}
func fmtDur(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	h := m / 60
	s := (d % time.Minute) / time.Second
	if s == 0 {
		return fmt.Sprintf("%d мин", m)
	}
	if m >= 60 {
		return fmt.Sprintf("%d час %d мин", h, m-60*h)
	}
	return fmt.Sprintf("%d мин %d сек", m, s)
}

func timeOfDay(t time.Time) time.Duration {
	return time.Duration(t.Hour())*time.Hour +
		time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second
}

func scheduleFor(t time.Time) []Period {
	if t.Weekday() == time.Monday {
		return mondaySchedule
	}
	return weekSchedule
}

type LessonEntry struct {
	Number  int
	Subject string
	Room    int
	Addi    bool
	IsIT    bool
	IsSE    bool
}

type ScheduleDay struct {
	Date    time.Time
	Entries []LessonEntry
}

const dateLayout = "02.01.06"

func ParseSchedule(r io.Reader) (ScheduleDay, error) {
	var day ScheduleDay
	dateParsed := false
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		additLes := false
		count := 0
		if line == "" {
			continue
		}
		if !dateParsed {
			date, err := time.Parse(dateLayout, line)
			if err != nil {
				return ScheduleDay{}, fmt.Errorf("строка %d: некорректная дата %q: %w", lineNum, line, err)
			}
			day.Date = date
			dateParsed = true
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			return ScheduleDay{}, fmt.Errorf("строка %d: ожидалось 3 поля, получено %d (%q)", lineNum, len(parts), line)
		}
		number, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return ScheduleDay{}, fmt.Errorf("строка %d: некорректный номер урока: %w", lineNum, err)
		}
		if count = utf8.RuneCountInString(strings.TrimSpace(parts[0])); count > 1 {
			additLes = true
			number = number / 10
		}
		room, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			return ScheduleDay{}, fmt.Errorf("строка %d: некорректный номер кабинета: %w", lineNum, err)
		}
		if additLes == false && count == 1 {
			day.Entries = append(day.Entries, LessonEntry{
				Number:  number,
				Subject: subjName(strings.TrimSpace(parts[1])),
				Room:    room,
				Addi:    false,
				IsIT:    it(strings.TrimSpace(parts[1])),
				IsSE:    se(strings.TrimSpace(parts[1])),
			})
		}
		if additLes == true {
			day.Entries = append(day.Entries, LessonEntry{
				Number:  number,
				Subject: subjName(strings.TrimSpace(parts[1])),
				Room:    room,
				Addi:    true,
				IsIT:    it(strings.TrimSpace(parts[1])),
				IsSE:    se(strings.TrimSpace(parts[1])),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return ScheduleDay{}, fmt.Errorf("ошибка чтения: %w", err)
	}
	if !dateParsed {
		return ScheduleDay{}, fmt.Errorf("дата не найдена")
	}
	return day, nil
}
func subjName(str string) string {
	st := strings.TrimSuffix(str, "(и.т.)")
	return strings.TrimSuffix(st, "(с.э.)")
}
func it(str string) bool {
	return strings.Contains(str, "(и.т.)")
}
func se(str string) bool {
	return strings.Contains(str, "(с.э.)")
}

type WeekSchedule struct {
	Days map[string]ScheduleDay `json:"days"` // ключ: "2006-01-02"
	mu   sync.Mutex
}

func NewWeekSchedule() *WeekSchedule {
	return &WeekSchedule{Days: make(map[string]ScheduleDay)}
}
func (w *WeekSchedule) SetDay(day ScheduleDay) {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := day.Date.Format("2006-01-02")
	w.Days[key] = day
}

func (w *WeekSchedule) GetDay(date time.Time) (ScheduleDay, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	day, ok := w.Days[date.Format("2006-01-02")]
	return day, ok
}

const schedulePath = "week_schedule.json"

func (w *WeekSchedule) Save(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	data, err := json.MarshalIndent(w.Days, "", "  ")
	if err != nil {
		return fmt.Errorf("маршалинг: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func LoadWeekSchedule(path string) (*WeekSchedule, error) {
	w := NewWeekSchedule()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return w, nil
	}
	if err != nil {
		return nil, fmt.Errorf("чтение файла: %w", err)
	}
	if err := json.Unmarshal(data, &w.Days); err != nil {
		return nil, fmt.Errorf("разбор JSON: %w", err)
	}
	return w, nil
}

func (d ScheduleDay) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Расписание на %s:\n", d.Date.Format(dateLayout))
	if len(d.Entries) == 0 {
		sb.WriteString("(пусто)")
		return sb.String()
	}
	for _, e := range d.Entries {
		fmt.Fprintf(&sb, "%d. %s (каб. %d)\n", e.Number, e.Subject, e.Room)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (w *WeekSchedule) GetDayByDate(dateStr string) (ScheduleDay, bool) {
	t, err := time.Parse(dateLayout, dateStr)
	if err != nil {
		return ScheduleDay{}, false
	}
	return w.GetDay(t)
}
func (w *WeekSchedule) DeleteDay(date time.Time) error {
	w.mu.Lock()
	key := date.Format("2006-01-02")
	if _, ok := w.Days[key]; !ok {
		w.mu.Unlock()
		return fmt.Errorf("дата %s не найдена", key)
	}
	delete(w.Days, key)
	w.mu.Unlock()
	return w.Save(path)
}
func dayFinished(day ScheduleDay, now time.Time) bool {
	periods := scheduleFor(now)
	if len(day.Entries) < len(periods) {
		periods = periods[:len(day.Entries)]
	}
	if len(periods) == 0 {
		return true
	}
	last := periods[len(periods)-1]
	return timeOfDay(now) >= last.End
}
