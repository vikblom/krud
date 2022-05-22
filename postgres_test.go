package krud_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/vikblom/krud"
)

const TEST_USER = "bill"
const DATE_FORMAT = "2006-01-02"

func CleanDatabase(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	url := os.Getenv("KRUD_TEST_DB_URL")
	if url == "" {
		t.Skip("KRUD_TEST_DB_URL not set, skipping test dependent on DB")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}

	// Nuke previous state
	_, err = db.Exec("DROP TABLE IF EXISTS users, objects, authors, books, events")
	if err != nil {
		t.Fatal(err)
	}

	// Re-run init file
	file, err := ioutil.ReadFile("./initdb/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(string(file))
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Logf("closing DB: %s", err)
		}
	}
	return db, cleanup
}

func MakeDate(t *testing.T, value string) krud.Date {
	t.Helper()

	date, err := time.Parse(DATE_FORMAT, value)
	if err != nil {
		t.Fatalf("make date: %s", err)
	}
	return krud.Date(date)
}

func TestUserAuthorized(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	_, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}
}

func TestUserUnauthorized(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	_, err := krud.NewAuditDB(context.Background(), pdb, "someone-else")
	if !errors.Is(err, krud.ErrUnauthorized) {
		t.Errorf("Expected err '%s' but got: %v", krud.ErrUnauthorized, err)
	}
}

func TestAuthorAdd(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	bday := MakeDate(t, "1982-01-25")
	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: bday}
	_, err = db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}
}

func TestAuthorGet(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	bday := MakeDate(t, "1982-01-25")
	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: bday}
	id, err := db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	woolf.ID = id

	expected := &woolf
	actual, err := db.GetAuthor(context.Background(), woolf.ID)
	if err != nil {
		t.Fatalf("get author: %s", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("wrong authors listed, expected %v but got %v", expected, actual)

	}
}

func TestAuthorGetAnother(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: MakeDate(t, "1982-01-25")}
	woolf.ID, err = db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	tolstoj := krud.Author{Name: "Leo Tolstoj", DateOfBirth: MakeDate(t, "1828-09-09")}
	tolstoj.ID, err = db.AddAuthor(context.Background(), tolstoj)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	expected := &tolstoj
	actual, err := db.GetAuthor(context.Background(), tolstoj.ID)
	if err != nil {
		t.Fatalf("get author: %s", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("wrong authors listed, expected %v but got %v", expected, actual)

	}
}

func TestAuthorGetMissing(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	_, err = db.GetAuthor(context.Background(), 123)
	if !errors.Is(err, krud.ErrDoesNotExist) {
		t.Errorf("Expected err '%s' but got: %v", krud.ErrDoesNotExist, err)
	}
}

func TestAuthorUpdateAndGet(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	bday := MakeDate(t, "1982-01-25")
	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: bday}
	id, err := db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	woolf.ID = id

	// Update to the same values.
	err = db.UpdateAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("update author: %s", err)
	}

	// Update to something else, but keeping ID.
	woolf.Name = "V. Woolf"
	woolf.DateOfBirth = MakeDate(t, "2000-01-01")
	err = db.UpdateAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("update author: %s", err)
	}

	expected := &woolf
	actual, err := db.GetAuthor(context.Background(), woolf.ID)
	if err != nil {
		t.Fatalf("get author: %s", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("wrong authors listed, expected %v but got %v", expected, actual)

	}
}

func TestAuthorAddThenDelete(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	bday := MakeDate(t, "1982-01-25")
	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: bday}
	id, err := db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}
	err = db.DeleteAuthor(context.Background(), id)
	if err != nil {
		t.Fatalf("delete author: %s", err)
	}
}

func TestAuthorAddThenGet(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	bday := MakeDate(t, "1982-01-25")
	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: bday}
	id, err := db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	// Expect to get back the same author, with ID assigned by DB.
	woolf.ID = id
	expected := &woolf

	actual, err := db.GetAuthor(context.Background(), id)
	if err != nil {
		t.Fatalf("get author: %s", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("wrong author, expected %v but got %v", expected, actual)

	}
}

func TestAuthorAddThenList(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	bday := MakeDate(t, "1982-01-25")
	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: bday}
	id, err := db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	// Expect to get back the same author, with ID assigned by DB.
	woolf.ID = id
	expected := []krud.Author{woolf}

	actual, err := db.AllAuthors(context.Background())
	if err != nil {
		t.Fatalf("listing authors: %s", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("wrong authors listed, expected %v but got %v", expected, actual)

	}
}

func TestAuthorDeleteMissing(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	err = db.DeleteAuthor(context.Background(), 1234)
	if !errors.Is(err, krud.ErrDoesNotExist) {
		t.Errorf("Expected err '%s' but got: %v", krud.ErrDoesNotExist, err)
	}
}

func TestAuthorChangeMissing(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	err = db.UpdateAuthor(context.Background(), krud.Author{ID: 1234, Name: "Virginia Woolf"})
	if !errors.Is(err, krud.ErrDoesNotExist) {
		t.Errorf("Expected err '%s' but got: %v", krud.ErrDoesNotExist, err)
	}
}

func TestAuthorConcurrentAddDelete(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		// TODO: Fail the test if subroutine errors.
		go func(i int) {
			defer wg.Done()

			adb, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
			if err != nil {
				t.Logf("helper opening db: %v", err)
				return
			}

			name := fmt.Sprintf("author_%d", i)
			id, err := adb.AddAuthor(context.Background(), krud.Author{Name: name})
			if err != nil {
				t.Logf("add author: %v", err)
				return
			}
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(10)))

			err = adb.DeleteAuthor(context.Background(), id)
			if err != nil {
				t.Logf("delete author: %s", err)
				return
			}
		}(i)
	}
	wg.Wait()

	adb, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}
	actual, err := adb.AllAuthors(context.Background())
	if err != nil {
		t.Fatalf("listing authors: %s", err)
	}
	if len(actual) != 0 {
		t.Errorf("expected no authors left but found: %v", actual)
	}
}

func TestBookAdd(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: MakeDate(t, "1982-01-25")}
	id, err := db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	book := krud.Book{Title: "To the Lighthouse", Published: MakeDate(t, "1927-05-05")}
	_, err = db.AddBook(context.Background(), id, book)
	if err != nil {
		t.Fatalf("add book: %v", err)
	}
}

func TestBookGet(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: MakeDate(t, "1982-01-25")}
	authorID, err := db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	book := krud.Book{Title: "To the Lighthouse", Published: MakeDate(t, "1927-05-05")}
	bookID, err := db.AddBook(context.Background(), authorID, book)
	if err != nil {
		t.Fatalf("add book: %v", err)
	}

	actual, err := db.GetBook(context.Background(), authorID, bookID)
	if err != nil {
		t.Fatalf("get book: %v", err)
	}

	book.ID = bookID
	expected := &book
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("wrong book, expected %v but got %v", expected, actual)

	}
}

func TestBookGetMissing(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	_, err = db.GetBook(context.Background(), 123, 456)
	if !errors.Is(err, krud.ErrDoesNotExist) {
		t.Errorf("Expected err '%s' but got: %v", krud.ErrDoesNotExist, err)
	}
}

// TODO: TestBooksGetAll

func TestBookAddThenDelete(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: MakeDate(t, "1982-01-25")}
	woolf.ID, err = db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	book := krud.Book{Title: "To the Lighthouse", Published: MakeDate(t, "1927-05-05")}
	book.ID, err = db.AddBook(context.Background(), woolf.ID, book)
	if err != nil {
		t.Fatalf("add book: %v", err)
	}

	err = db.DeleteBook(context.Background(), woolf.ID, book.ID)
	if err != nil {
		t.Fatalf("delete book: %s", err)
	}
}

func TestBookDeleteWrongAuthorID(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: MakeDate(t, "1982-01-25")}
	woolf.ID, err = db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	book := krud.Book{Title: "To the Lighthouse", Published: MakeDate(t, "1927-05-05")}
	book.ID, err = db.AddBook(context.Background(), woolf.ID, book)
	if err != nil {
		t.Fatalf("add book: %v", err)
	}

	err = db.DeleteBook(context.Background(), 123, book.ID)
	if !errors.Is(err, krud.ErrDoesNotExist) {
		t.Errorf("Expected err '%s' but got: %v", krud.ErrDoesNotExist, err)
	}
}

func TestBookDeleteWrongBookID(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: MakeDate(t, "1982-01-25")}
	woolf.ID, err = db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	book := krud.Book{Title: "To the Lighthouse", Published: MakeDate(t, "1927-05-05")}
	book.ID, err = db.AddBook(context.Background(), woolf.ID, book)
	if err != nil {
		t.Fatalf("add book: %v", err)
	}

	err = db.DeleteBook(context.Background(), woolf.ID, 456)
	if !errors.Is(err, krud.ErrDoesNotExist) {
		t.Errorf("Expected err '%s' but got: %v", krud.ErrDoesNotExist, err)
	}
}

// TODO: Audit log tests. Test that events contain the correct info.

func TestAuditEventsAfterAddingAuthors(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: MakeDate(t, "1982-01-25")}
	_, err = db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	tolstoj := krud.Author{Name: "Leo Tolstoj", DateOfBirth: MakeDate(t, "1828-09-09")}
	_, err = db.AddAuthor(context.Background(), tolstoj)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	strindberg := krud.Author{Name: "August Strindberg", DateOfBirth: MakeDate(t, "1849-01-22")}
	_, err = db.AddAuthor(context.Background(), strindberg)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	events, err := db.QueryEvents(context.Background())
	if err != nil {
		t.Fatalf("query events: %s", err)
	}
	// 4 expected = 1 auth check + 3 books.
	if len(events) != 4 {
		t.Fatalf("expected 4 events but got: %d", len(events))
	}
}

func TestAuditEventsFilterOnTime(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: MakeDate(t, "1982-01-25")}
	_, err = db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	tolstoj := krud.Author{Name: "Leo Tolstoj", DateOfBirth: MakeDate(t, "1828-09-09")}
	_, err = db.AddAuthor(context.Background(), tolstoj)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	now := time.Now() // Divide events into before/after here.

	strindberg := krud.Author{Name: "August Strindberg", DateOfBirth: MakeDate(t, "1849-01-22")}
	_, err = db.AddAuthor(context.Background(), strindberg)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	// Check that events can be queried on time as expected.

	eventsAfter, err := db.QueryEvents(context.Background(), krud.EventsAfter(now))
	if err != nil {
		t.Fatalf("query events: %s", err)
	}
	// 2 expected = 1 auth check + 2 books.
	if len(eventsAfter) != 1 {
		t.Fatalf("expected 1 events but got: %d", len(eventsAfter))
	}

	eventsBefore, err := db.QueryEvents(context.Background(), krud.EventsBefore(now))
	if err != nil {
		t.Fatalf("query events: %s", err)
	}
	// 1 expected = just 3 book.
	if len(eventsBefore) != 3 {
		t.Fatalf("expected 3 events but got: %d", len(eventsBefore))
	}
}

func TestAuditEventsFilterOnTimeWindow(t *testing.T) {
	pdb, closer := CleanDatabase(t)
	defer closer()

	db, err := krud.NewAuditDB(context.Background(), pdb, TEST_USER)
	if err != nil {
		t.Fatalf("helper opening db: %v", err)
	}

	woolf := krud.Author{Name: "Virginia Woolf", DateOfBirth: MakeDate(t, "1982-01-25")}
	_, err = db.AddAuthor(context.Background(), woolf)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	start := time.Now()

	tolstoj := krud.Author{Name: "Leo Tolstoj", DateOfBirth: MakeDate(t, "1828-09-09")}
	_, err = db.AddAuthor(context.Background(), tolstoj)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	end := time.Now()

	strindberg := krud.Author{Name: "August Strindberg", DateOfBirth: MakeDate(t, "1849-01-22")}
	_, err = db.AddAuthor(context.Background(), strindberg)
	if err != nil {
		t.Fatalf("add author: %v", err)
	}

	// Check that events can be queried on time as expected.
	eventsAfter, err := db.QueryEvents(context.Background(), krud.EventsAfter(start), krud.EventsBefore(end))
	if err != nil {
		t.Fatalf("query events: %s", err)
	}
	if len(eventsAfter) != 1 {
		t.Fatalf("expected 1 events but got: %d", len(eventsAfter))
	}
}
