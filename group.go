package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type Group int8

const (
	Empty Group = iota
	SocEco
	InfTec
)

type Birthday struct {
	Day   int `json:"day"`
	Month int `json:"month"`
	Year  int `json:"year,omitempty"`
}

func (b Birthday) IsSet() bool { return b.Day != 0 && b.Month != 0 }
func parseBirthday(s string) (Birthday, error) {
	t, err := time.Parse("02.01.2006", strings.TrimSpace(s))
	if err != nil {
		return Birthday{}, err
	}
	if t.After(time.Now()) || t.Year() < 1900 {
		return Birthday{}, errors.New("нереальный год")
	}
	return Birthday{Day: t.Day(), Month: int(t.Month()), Year: t.Year()}, nil
}

type Classmate struct {
	Group        Group     `json:"group"`
	Name         string    `json:"name,omitempty"`
	Notify       bool      `json:"notify,omitempty"`
	Announcement string    `json:"announcement,omitempty"`
	Innovations  bool      `json:"innovations,omitempty"`
	Duty         bool      `json:"duty,omitempty"`
	LastRemember time.Time `json:"last_remember,omitzero"`
	Birthday     Birthday  `json:"birthday,omitzero"`
	SettingsPage int8      `json:"settings_page,omitzero"`
	Configured   bool      `json:"configured,omitempty"`
}

type ClassRegistry struct {
	Users map[int64]Classmate
	mu    sync.Mutex
}

func (c *Classmate) UnmarshalJSON(b []byte) error {
	var g Group
	if err := json.Unmarshal(b, &g); err == nil {
		*c = Classmate{Group: g}
		return nil
	}
	type plain Classmate
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*c = Classmate(p)
	return nil
}

func (c *ClassRegistry) GetGroup(id int64) (Group, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	u, ok := c.Users[id]
	return u.Group, ok
}

func (c *ClassRegistry) SetGroup(id int64, g Group) {
	c.Update(id, func(u *Classmate) { u.Group = g })
}

func (c *ClassRegistry) Update(id int64, fn func(*Classmate)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	u := c.Users[id]
	fn(&u)
	c.Users[id] = u
	return c.saveLocked(claPath)
}

func (c *ClassRegistry) Save(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked(path)
}

func (c *ClassRegistry) saveLocked(path string) error {
	data, err := json.MarshalIndent(c.Users, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *ClassRegistry) Get(id int64) (Classmate, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	u, ok := c.Users[id]
	return u, ok
}

func (c *ClassRegistry) IDs() []int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]int64, 0, len(c.Users))
	for id := range c.Users {
		ids = append(ids, id)
	}
	return ids
}

func LoadClassRegistry(path string) (*ClassRegistry, error) {
	c := &ClassRegistry{Users: make(map[int64]Classmate)}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &c.Users); err != nil {
		return nil, fmt.Errorf("разбор %s: %w", path, err)
	}
	if c.Users == nil {
		c.Users = make(map[int64]Classmate)
	}
	return c, nil
}

func Check(chatid int64) bool {
	g, ok := Groups.GetGroup(chatid)
	return ok && g != Empty
}

func CheckConfigured(chatid int64) bool {
	cls, ok := Groups.Get(chatid)
	return ok && cls.Configured
}

func (g Group) String() string {
	switch g {
	case Empty:
		return "Empty"
	case SocEco:
		return "SocEco"
	case InfTec:
		return "InfTec"
	default:
		return "Unknown"
	}
}
func (g Group) RuString() string {
	switch g {
	case Empty:
		return "Нет группы"
	case SocEco:
		return "Социально-экономический"
	case InfTec:
		return "Информационно-технический"
	default:
		return "хз"
	}
}
