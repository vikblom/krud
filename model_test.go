package krud_test

import (
	"strings"
	"testing"
	"time"

	"github.com/vikblom/krud"
)

func TestAuthorValid(t *testing.T) {

	tests := []string{
		"Foo Bar",
		"foo bar",
		"foo",
		"f",
		"Göran",
		"Lászlo",
		"E. M. Forster",
	}

	for _, tt := range tests {
		a := krud.Author{Name: tt, DateOfBirth: krud.Date(time.Now())}
		err := a.Validate()
		if err != nil {
			t.Errorf("expected valid author '%+v' but got err: '%v'", a, err)
		}
	}
}

func TestAuthorInvalid(t *testing.T) {

	tests := []struct {
		Name   string
		Expect string
	}{
		{Name: "", Expect: "name empty"},
		{Name: "1", Expect: "unexpected rune"},
		{Name: "Foo#Bar", Expect: "unexpected rune"},
	}

	for _, tt := range tests {
		a := krud.Author{Name: tt.Name, DateOfBirth: krud.Date(time.Now())}
		err := a.Validate()

		if err == nil || !strings.Contains(err.Error(), tt.Expect) {
			t.Errorf("expected err matching '%s' but got: '%v'", tt.Expect, err)
		}
	}

}
