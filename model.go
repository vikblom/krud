package krud

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Date is borrowed from:
// https://stackoverflow.com/questions/45303326/how-to-parse-non-standard-time-format-from-json
// Overrides time.Time with only year, month, day in JSON representation.
type Date time.Time

// Implement Marshaler and Unmarshaler interface
func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d).Format("2006-01-02"))
}

// Maybe a Format function for printing your date
func (d Date) Format(s string) string {
	t := time.Time(d)
	return t.Format(s)
}

type Author struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DateOfBirth Date   `json:"dateofbirth"`
}

// Validate does basic sanity checking of this Author.
// But there is always:
// https://www.kalzumeus.com/2010/06/17/falsehoods-programmers-believe-about-names/
func (a *Author) Validate() error {

	if len(a.Name) == 0 {
		return errors.New("name empty")
	}
	for _, r := range a.Name {
		if unicode.IsLetter(r) {
			continue
		}
		if r == ' ' || r == '.' {
			continue
		}
		return fmt.Errorf("name contains unexpected rune: %c", r)

	}

	if time.Time(a.DateOfBirth).IsZero() {
		return errors.New("birthdate before the start of civilization")
	}

	return nil
}

type Book struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Published Date   `json:"published"`
}

func (b *Book) Validate() error {

	if len(b.Title) == 0 {
		return errors.New("title empty")
	}

	if time.Time(b.Published).IsZero() {
		return errors.New("published before the start of civilization")
	}

	return nil
}
