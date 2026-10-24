package main

import "time"

// lesson schedule;

var mondayNfridaySchedule = []Period{
	P(8, 30, 9, 10),
	P(9, 20, 10, 00),
	P(10, 15, 10, 55),
	P(11, 15, 11, 55),
	P(12, 10, 12, 50),
	P(13, 5, 13, 45),
	P(13, 50, 14, 25),
	P(14, 35, 15, 15),
}
var thursdaySchedule = []Period{
	P(8, 30, 9, 10),
	P(9, 20, 10, 00),
	P(10, 15, 10, 55),
	P(11, 15, 11, 55),
	P(12, 10, 12, 50),
	P(13, 5, 13, 45),
	P(13, 50, 14, 20),
	P(14, 30, 15, 10),
}

var weekSchedule = []Period{
	P(8, 30, 9, 10),
	P(9, 20, 10, 00),
	P(10, 15, 10, 55),
	P(11, 15, 11, 55),
	P(12, 10, 12, 50),
	P(13, 5, 13, 45),
	P(13, 50, 14, 30),
	P(14, 40, 15, 20),
}

// class bell schedule

var mondayBase = ScheduleDay{
	Date: time.Date(2026, time.January, 5, 0, 0, 0, 0, time.Local),
	Base: true,
	Entries: []LessonEntry{
		{Number: 1, Subject: "Разговоры о важном", Room: 28},
		{Number: 2, Subject: "Алгебра", Room: 42},
		{Number: 3, Subject: "Алгебра", Room: 42},
		{Number: 4, Subject: "Русский язык", Room: 12},
		{Number: 5, Subject: "Русский язык", Room: 12},
		{Number: 6, Subject: "История", Room: 22, Group: SocEco},
		{Number: 7, Subject: "История", Room: 22, Group: SocEco},
	},
}

var tuesdayBase = ScheduleDay{
	Date: time.Date(2026, time.January, 6, 0, 0, 0, 0, time.Local),
	Base: true,
	Entries: []LessonEntry{
		{Number: 1, Subject: "Физика", Room: 29},
		{Number: 2, Subject: "Английский язык", Room: 43},
		{Number: 3, Subject: "Обществознание", Room: 22, Group: SocEco},
		{Number: 3, Subject: "Информатика", Room: 23, Group: InfTec},
		{Number: 4, Subject: "Информатика", Room: 19, Group: InfTec},
		{Number: 4, Subject: "Обществознание", Room: 22, Group: SocEco},
		{Number: 5, Subject: "Геометрия", Room: 42},
		{Number: 6, Subject: "Физкультура"},
		{Number: 7, Subject: "История", Room: 22, Group: InfTec},
	},
}

var wednesdayBase = ScheduleDay{
	Date: time.Date(2026, time.January, 7, 0, 0, 0, 0, time.Local),
	Base: true,
	Entries: []LessonEntry{
		{Number: 1, Subject: "Литература", Room: 12},
		{Number: 2, Subject: "Литература", Room: 12},
		{Number: 3, Subject: "Алгебра", Room: 42},
		{Number: 4, Subject: "История", Room: 22, Group: SocEco},
		{Number: 4, Subject: "Информатика (Эл.)", Room: 19, Group: InfTec},
		{Number: 5, Subject: "Информатика (Эл.)", Room: 19, Group: InfTec},
		{Number: 5, Subject: "История", Room: 22, Group: SocEco},
		{Number: 6, Subject: "История", Room: 22, Group: InfTec},
		{Number: 6, Subject: "Индивид. проект", Room: 48, Group: SocEco},
		{Number: 7, Subject: "Обществознание", Room: 22, Group: InfTec},
	},
}

var thursdayBase = ScheduleDay{
	Date: time.Date(2026, time.January, 8, 0, 0, 0, 0, time.Local),
	Base: true,
	Entries: []LessonEntry{
		{Number: 1, Subject: "ОБЗР", Room: 21},
		{Number: 2, Subject: "Вероятность и статистика", Room: 42},
		{Number: 3, Subject: "Английский язык", Room: 11},
		{Number: 4, Subject: "География", Room: 48},
		{Number: 5, Subject: "Физика", Room: 29},
		{Number: 6, Subject: "Россия мои горизонты", Room: 28},
		{Number: 7, Subject: "Математика (эл.)", Room: 42},
	},
}

var fridayBase = ScheduleDay{
	Date: time.Date(2026, time.January, 9, 0, 0, 0, 0, time.Local),
	Base: true,
	Entries: []LessonEntry{
		{Number: 1, Subject: "Алгебра", Room: 42},
		{Number: 2, Subject: "Литература", Room: 12},
		{Number: 3, Subject: "Русский язык (эл.)", Room: 12},
		{Number: 4, Subject: "Информатика", Room: 19, Group: SocEco},
		{Number: 4, Subject: "Обществознание", Room: 42, Group: InfTec},
		{Number: 5, Subject: "Обществознание", Room: 42, Group: SocEco},
		{Number: 5, Subject: "Информатика", Room: 19, Group: InfTec},
		{Number: 6, Subject: "Обществознание", Room: 42, Group: SocEco},
		{Number: 6, Subject: "Информатика", Room: 19, Group: InfTec},
		{Number: 7, Subject: "Английский язык", Room: 11},
	},
}

var saturdayBase = ScheduleDay{
	Date: time.Date(2026, time.January, 10, 0, 0, 0, 0, time.Local),
	Base: true,
	Entries: []LessonEntry{
		{Number: 1, Subject: "Химия", Room: 20},
		{Number: 2, Subject: "Биология", Room: 28},
		{Number: 3, Subject: "Геометрия", Room: 28},
		{Number: 4, Subject: "Геометрия", Room: 28},
		{Number: 5, Subject: "Физкультура"},
		{Number: 6, Subject: "Индивид. проект", Room: 23, Group: InfTec},
		{Number: 6, Subject: "История (эл.)", Room: 22, Group: SocEco},
	},
}
