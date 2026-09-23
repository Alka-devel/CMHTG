package main

import (
	"context"
	"sync"

	"github.com/mymmrac/telego"
)

type Waiter struct {
	mu      sync.Mutex
	waiting map[telego.ChatID]chan telego.Update
}

func NewWaiter() *Waiter {
	return &Waiter{waiting: make(map[telego.ChatID]chan telego.Update)}
}

func (w *Waiter) Dispatch(chatID telego.ChatID, update telego.Update) bool {
	w.mu.Lock()
	ch, ok := w.waiting[chatID]
	w.mu.Unlock()
	if !ok {
		return false
	}
	ch <- update
	return true
}

func (w *Waiter) WaitForMessage(ctx context.Context, chatID telego.ChatID) (telego.Update, error) {
	ch := make(chan telego.Update, 1)
	w.mu.Lock()
	w.waiting[chatID] = ch
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.waiting, chatID)
		w.mu.Unlock()
	}()

	select {
	case upd := <-ch:
		return upd, nil
	case <-ctx.Done():
		return telego.Update{}, ctx.Err()
	}
}
