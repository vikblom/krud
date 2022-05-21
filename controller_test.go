package krud_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/vikblom/krud"
)

type MockDatabase struct {
	latestAuthor int64
	authors      map[int64]krud.Author

	latestBook int64
	books      map[int64]krud.Book
}

func EmptyMock() *MockDatabase {
	return &MockDatabase{1, map[int64]krud.Author{}, 1, map[int64]krud.Book{}}
}

func (mock *MockDatabase) AddAuthor(ctx context.Context, author krud.Author) (id int64, err error) {

	return 0, nil
}

func (mock *MockDatabase) GetAuthor(ctx context.Context, id int64) (author *krud.Author, err error) {
	return &krud.Author{}, nil
}

func (mock *MockDatabase) UpdateAuthor(ctx context.Context, author krud.Author) (err error) {
	return nil
}

func (mock *MockDatabase) AllAuthors(ctx context.Context) (authors []krud.Author, err error) {
	return []krud.Author{}, nil
}

func (mock *MockDatabase) DeleteAuthor(ctx context.Context, id int64) (err error) {
	return nil
}

func (mock *MockDatabase) AddBook(ctx context.Context, author int64, book krud.Book) (id int64, err error) {
	mock.latestBook += 1
	mock.books[mock.latestBook] = book
	return mock.latestBook, nil
}

func (mock *MockDatabase) GetBook(ctx context.Context, authorID, bookID int64) (book *krud.Book, err error) {
	return &krud.Book{}, nil
}

func (mock *MockDatabase) UpdateBook(ctx context.Context, authorID int64, book krud.Book) (err error) {
	return nil
}

func (mock *MockDatabase) AllBooks(ctx context.Context) (books []krud.Book, err error) {
	return nil, nil
}

func (mock *MockDatabase) DeleteBook(ctx context.Context, authorID, bookID int64) (err error) {
	return nil
}

func (mock *MockDatabase) QueryEvents(ctx context.Context, filters ...krud.Filter) (events []krud.Event, err error) {
	return nil, nil
}

func (mock *MockDatabase) Dial(ctx context.Context, user string) (krud.Databaser, error) {
	return mock, nil
}

func checkStatusCode(t *testing.T, r *http.Response, expected int) {
	t.Helper()
	if r.StatusCode != expected {
		t.Errorf("expected status code '%v' but got: '%v'", expected, r.StatusCode)
	}
}

// apiJSON formats string like the API promises.
// Indented with 4 spaces and a trailing newline.
func apiJSON(in string) string {
	buf := bytes.Buffer{}
	json.Indent(&buf, []byte(in), "", "    ")
	buf.WriteByte('\n')
	return string(buf.Bytes())
}

func TestRequestGetAuthorsEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/authors", nil)
	w := httptest.NewRecorder()

	r := mux.NewRouter()
	mock := EmptyMock()
	log, _ := test.NewNullLogger()
	krud.NewController(log, r, mock)
	r.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var m map[string]string
	json.Unmarshal(data, &m)

	checkStatusCode(t, resp, http.StatusOK)
	expected := apiJSON("[]")
	if string(data) != expected {
		t.Errorf("expected '%v' but got: '%v'", expected, string(data))
	}
}

// req.Header.Set("Content-Type", "application/json")

// ParseIDFromJSON grabs "id" (assumed to be) number from a buffer.
func ParseIDFromJSON(t *testing.T, b []byte) int64 {
	t.Helper()

	var m map[string]interface{}
	json.Unmarshal(b, &m)

	tmp, ok := m["id"]
	if !ok {
		t.Fatalf("found no 'id' in: %s", string(b))
	}
	id, ok := tmp.(float64)
	if !ok {
		t.Fatal("id not a number")
	}
	return int64(id)
}

func TestRequestPostBook(t *testing.T) {
	body := strings.NewReader(`{"title":"foo", "published":"1970-01-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/authors/5/books", body)
	w := httptest.NewRecorder()

	r := mux.NewRouter()
	mock := EmptyMock()
	log, _ := test.NewNullLogger()
	krud.NewController(log, r, mock)
	r.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	checkStatusCode(t, resp, http.StatusCreated)
	actual, ok := mock.books[ParseIDFromJSON(t, data)]
	if !ok {
		t.Error("POST-ed book did not reach database")
	}
	_ = actual
	// TODO: Compare response and db book.
}
