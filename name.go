package main

import (
	"errors"
	"strings"
	"unicode"
)

const vowels = "аеёиоуыэюя"

func cleanName(s string) (string, error) {
	words := strings.Fields(s)
	if len(words) == 0 || len(words) > 2 {
		return "", errors.New("напиши имя (можно с фамилией), одно или два слова")
	}
	for i, w := range words {
		c, err := cleanWord(w)
		if err != nil {
			return "", err
		}
		words[i] = c
	}
	return strings.Join(words, " "), nil
}

func cleanWord(w string) (string, error) {
	r := []rune(strings.ToLower(w))
	if len(r) < 2 || len(r) > 20 {
		return "", errors.New("каждое слово должно быть от 2 до 20 букв")
	}
	hasVowel, run := false, 1
	for i, c := range r {
		switch {
		case c == '-':
			if i == 0 || i == len(r)-1 || r[i-1] == '-' {
				return "", errors.New("дефис может стоять только внутри слова")
			}
		case (c >= 'а' && c <= 'я') || c == 'ё':
		default:
			return "", errors.New("только русские буквы и дефис")
		}
		if strings.ContainsRune(vowels, c) {
			hasVowel = true
		}
		if i > 0 && c == r[i-1] {
			if run++; run >= 3 {
				return "", errors.New("три одинаковые буквы подряд — это не имя")
			}
		} else {
			run = 1
		}
	}
	if !hasVowel {
		return "", errors.New("в имени должна быть хотя бы одна гласная")
	}
	parts := strings.Split(string(r), "-")
	for i, p := range parts {
		pr := []rune(p)
		pr[0] = unicode.ToUpper(pr[0])
		parts[i] = string(pr)
	}
	return strings.Join(parts, "-"), nil
}
func fixNames() {
	for _, id := range Groups.IDs() {
		Groups.Update(id, func(c *Classmate) {
			if c.Name == "" {
				return
			}
			if n, err := cleanName(c.Name); err == nil {
				c.Name = n
			} else {
				c.Name, c.Configured, c.SettingsPage = "", false, 0
			}
		})
	}
}
