package main

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
)

type Group int8

const (
	Empty Group = iota
	SocEco
	InfTec
)

type ClassRegistry struct {
	Group map[int64]Group `json:"groups"`
	mu    sync.Mutex
}

func (c *ClassRegistry) SetGroup(chatID int64, g Group) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Group[chatID] = g
}

func (c *ClassRegistry) GetGroup(chatID int64) (Group, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g, ok := c.Group[chatID]
	return g, ok
}

func (c *ClassRegistry) Save(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := json.MarshalIndent(c.Group, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
func LoadClassRegistry(path string) (*ClassRegistry, error) {
	c := &ClassRegistry{Group: make(map[int64]Group)}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &c.Group); err != nil {
		return nil, err
	}
	return c, nil
}

func Check(chatid int64) bool {
	_, nah := Groups.GetGroup(chatid)
	return nah
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
