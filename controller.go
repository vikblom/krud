package krud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// NOTE: Would probably have been much easier to have "AllowedUser" as
// a normal method on the DB and then use a single *sql.DB across
// all connections. A middleware could ensure that each request is checked.
// Now its part of the creation of a handle to the underlying db layer.
// Although this approach has the upside of tying together the user performing queries!

// Databaser should have a better name.
type Databaser interface {
	AddAuthor(ctx context.Context, author Author) (id int64, err error)
	GetAuthor(ctx context.Context, id int64) (author *Author, err error)
	UpdateAuthor(ctx context.Context, author Author) (err error)
	AllAuthors(ctx context.Context) (authors []Author, err error)
	DeleteAuthor(ctx context.Context, id int64) (err error)
	AddBook(ctx context.Context, author int64, book Book) (id int64, err error)
	GetBook(ctx context.Context, authorID, bookID int64) (book *Book, err error)
	UpdateBook(ctx context.Context, authorID int64, book Book) (err error)
	AllBooks(ctx context.Context) (books []Book, err error)
	DeleteBook(ctx context.Context, authorID, bookID int64) (err error)
	QueryEvents(ctx context.Context, filters ...Filter) (events []Event, err error)
}

// Dialer lets us setup and API that does not have to know what kind of Database is used.
// Enabled injecting a mock when testing.
type Dialer interface {
	Dial(context.Context, string) (Databaser, error)
}

type DialFunc func(context.Context, string) (Databaser, error)

func (df DialFunc) Dial(ctx context.Context, user string) (Databaser, error) {
	return df(ctx, user)
}

// Controller wires up the endpoints.
type Controller struct {
	// dial is used to create handles to some Databaser.
	dial Dialer
	// log is an injected logger.
	log *log.Logger
}

type contextKrudDatabaser struct{}

// NewController adds endpoints under r and hooks them up to the resources behind dial.
func NewController(log *log.Logger, r *mux.Router, dial Dialer) *Controller {

	c := Controller{
		dial: dial,
		log:  log,
	}

	// Make sure any request is from an approved user.
	r.Use(c.AuthMiddleware)

	r.HandleFunc("/authors", c.CreateAuthor).Methods(http.MethodPost)
	r.HandleFunc("/authors", c.ReadAuthor).Methods(http.MethodGet) // Two get routes for w/ and w/o id.
	r.HandleFunc("/authors/{authorID:[0-9]+}", c.ReadAuthor).Methods(http.MethodGet)
	r.HandleFunc("/authors/{authorID:[0-9]+}", c.UpdateAuthor).Methods(http.MethodPatch)
	r.HandleFunc("/authors/{authorID:[0-9]+}", c.DeleteAuthor).Methods(http.MethodDelete)

	r.HandleFunc("/authors/{authorID:[0-9]+}/books", c.CreateBook).Methods(http.MethodPost)
	r.HandleFunc("/authors/{authorID:[0-9]+}/books", c.ReadBook).Methods(http.MethodGet) // Two get routes for w/ and w/o id.
	r.HandleFunc("/authors/{authorID:[0-9]+}/books/{bookID:[0-9]+}", c.ReadBook).Methods(http.MethodGet)
	r.HandleFunc("/authors/{authorID:[0-9]+}/books/{bookID:[0-9]+}", c.UpdateBook).Methods(http.MethodPatch)
	r.HandleFunc("/authors/{authorID:[0-9]+}/books/{bookID:[0-9]+}", c.DeleteBook).Methods(http.MethodDelete)

	r.HandleFunc("/events", c.Events).Methods(http.MethodPost)

	return &c
}

func (api Controller) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Even more basic than r.BasicAuth()...
		user := r.Header.Get("user")

		// Needs to create DB for this user...
		db, err := api.dial.Dial(r.Context(), user)
		if err != nil {
			api.log.Infof("auth rejected '%s' (%s) access to %s", user, r.RemoteAddr, r.RequestURI)
			http.Error(w, "specify approved user in header", http.StatusUnauthorized)
			return
		}
		api.log.Infof("auth approved '%s' (%s) access to %s", user, r.RemoteAddr, r.RequestURI)

		// Propagate db decorated for this approved user.
		ctx := context.WithValue(r.Context(), contextKrudDatabaser{}, db)
		rr := r.WithContext(ctx)
		next.ServeHTTP(w, rr)
	})
}

// WriteJsonError pack cause in a json body of http response with code set in header.
func WriteJsonError(w http.ResponseWriter, cause error, code int) {
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	// Use anonymous struct to put error string a json.
	err := enc.Encode(struct {
		Error string `json:"error"`
	}{Error: cause.Error()})
	if err != nil {
		// FIXME: What is the fallback error handling?
		return
	}
}

func WriteJson(w http.ResponseWriter, item interface{}, code int) {
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	err := enc.Encode(item)
	if err != nil {
		// FIXME: What is the fallback error handling?
		return
	}
}

func (api *Controller) CreateAuthor(w http.ResponseWriter, r *http.Request) {

	author := Author{}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&author)
	if err != nil {
		WriteJsonError(w, fmt.Errorf("json decode body: %w", err), http.StatusBadRequest)
		return
	}

	err = author.Validate()
	if err != nil {
		WriteJsonError(w, err, http.StatusBadRequest)
		return
	}

	db, ok := r.Context().Value(contextKrudDatabaser{}).(Databaser)
	if !ok {
		WriteJson(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}

	author.ID, err = db.AddAuthor(r.Context(), author)
	if err != nil {
		WriteJson(w, err, http.StatusInternalServerError)
		return
	}

	WriteJson(w, author, http.StatusCreated)
}

func (api *Controller) ReadAuthor(w http.ResponseWriter, r *http.Request) {

	db, ok := r.Context().Value(contextKrudDatabaser{}).(Databaser)
	if !ok {
		WriteJson(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}

	if _, ok := mux.Vars(r)["authorID"]; ok { // List specific
		id, err := GetIntFromRequest(r, "authorID")
		if err != nil {
			WriteJsonError(w, err, http.StatusInternalServerError)
			return
		}

		author, err := db.GetAuthor(r.Context(), int64(id))
		if err != nil {
			if errors.Is(err, ErrDoesNotExist) {
				WriteJsonError(w, err, http.StatusNotFound)
				return
			}
			WriteJsonError(w, err, http.StatusInternalServerError)
			return
		}
		WriteJson(w, author, http.StatusOK)

	} else { // List all
		authors, err := db.AllAuthors(r.Context())
		if err != nil {
			WriteJsonError(w, err, http.StatusInternalServerError)
			return
		}
		// Always return some json.
		if authors == nil {
			authors = []Author{}
		}
		WriteJson(w, authors, http.StatusOK)
	}
}

func (api *Controller) DeleteAuthor(w http.ResponseWriter, r *http.Request) {
	db, ok := r.Context().Value(contextKrudDatabaser{}).(Databaser)
	if !ok {
		WriteJson(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}

	id, err := GetIntFromRequest(r, "authorID")
	if err != nil {
		WriteJsonError(w, err, http.StatusInternalServerError)
		return
	}

	err = db.DeleteAuthor(r.Context(), int64(id))
	if err != nil {
		if errors.Is(err, ErrDoesNotExist) {
			WriteJsonError(w, err, http.StatusNotFound)
			return
		}
		WriteJsonError(w, err, http.StatusInternalServerError)
		return
	}
	// Should we return repr of deleted resource?
	w.WriteHeader(http.StatusNoContent)
}

func (api *Controller) UpdateAuthor(w http.ResponseWriter, r *http.Request) {

	id, err := GetIntFromRequest(r, "authorID")
	if err != nil {
		WriteJsonError(w, err, http.StatusInternalServerError)
		return
	}

	db, ok := r.Context().Value(contextKrudDatabaser{}).(Databaser)
	if !ok {
		WriteJson(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}

	// This is a hack because of limited HTTP and database APIs.
	// DB update is all or nothing, so write request changes onto existing record.
	author, err := db.GetAuthor(r.Context(), int64(id))
	if err != nil {
		if errors.Is(err, ErrDoesNotExist) {
			WriteJsonError(w, err, http.StatusNotFound)
			return
		}
		WriteJsonError(w, err, http.StatusInternalServerError)
		return
	}
	// Hack #2, use anon struct to the request cannot contain som ID conflicting with the URL.
	changes := struct {
		Name        string `json:"name"`
		DateOfBirth Date   `json:"dateofbirth"`
	}{}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err = dec.Decode(&changes)
	if err != nil {
		WriteJsonError(w, fmt.Errorf("json decode body: %w", err), http.StatusBadRequest)
		return
	}
	// Only override fields which are in the request.
	if changes.Name != "" {
		author.Name = changes.Name
	}
	if !time.Time(changes.DateOfBirth).IsZero() {
		author.DateOfBirth = changes.DateOfBirth
	}

	// This is a bit brittle. We block changes if the author is invalid, but this author is
	// a combination of the request and current state. If the invalid-ness comes from state
	// (after something like a policy change or schema migration) we cannot fix anything through
	// the api.
	err = author.Validate()
	if err != nil {
		WriteJsonError(w, err, http.StatusBadRequest)
		return
	}

	err = db.UpdateAuthor(r.Context(), *author)
	if err != nil {
		if errors.Is(err, ErrDoesNotExist) {
			WriteJsonError(w, err, http.StatusNotFound)
			return
		}
		WriteJson(w, err, http.StatusInternalServerError)
		return
	}

	WriteJson(w, author, http.StatusOK)
}

func GetIntFromRequest(r *http.Request, key string) (int, error) {
	vars := mux.Vars(r)
	id, ok := vars[key]
	if !ok {
		return 0, fmt.Errorf("handler did not populate: %s", key)
	}
	return strconv.Atoi(id)
}

func (api *Controller) CreateBook(w http.ResponseWriter, r *http.Request) {
	db, ok := r.Context().Value(contextKrudDatabaser{}).(Databaser)
	if !ok {
		WriteJson(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}

	authorID, err := GetIntFromRequest(r, "authorID")
	if err != nil {
		WriteJsonError(w, err, http.StatusInternalServerError)
		return
	}

	book := Book{}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err = dec.Decode(&book)
	if err != nil {
		WriteJsonError(w, fmt.Errorf("json decode body: %w", err), http.StatusBadRequest)
		return
	}

	err = book.Validate()
	if err != nil {
		WriteJsonError(w, err, http.StatusBadRequest)
		return
	}

	book.ID, err = db.AddBook(r.Context(), int64(authorID), book)
	if err != nil {
		WriteJson(w, err, http.StatusInternalServerError)
		return
	}

	WriteJson(w, book, http.StatusCreated)
}

func (api *Controller) ReadBook(w http.ResponseWriter, r *http.Request) {
	db, ok := r.Context().Value(contextKrudDatabaser{}).(Databaser)
	if !ok {
		WriteJson(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}

	authorID, err := GetIntFromRequest(r, "authorID")
	if err != nil {
		WriteJsonError(w, err, http.StatusInternalServerError)
	}

	if _, ok := mux.Vars(r)["bookID"]; ok { // List specific
		bookID, err := GetIntFromRequest(r, "bookID")
		if err != nil {
			WriteJsonError(w, err, http.StatusInternalServerError)
			return
		}

		book, err := db.GetBook(r.Context(), int64(authorID), int64(bookID))
		if err != nil {
			if errors.Is(err, ErrDoesNotExist) {
				WriteJsonError(w, err, http.StatusNotFound)
				return
			}
			WriteJsonError(w, err, http.StatusInternalServerError)
			return
		}
		WriteJson(w, book, http.StatusOK)

	} else { // List all
		books, err := db.AllBooks(r.Context())
		if err != nil {
			WriteJsonError(w, err, http.StatusInternalServerError)
			return
		}
		// Always return some json.
		if books == nil {
			books = []Book{}
		}
		WriteJson(w, books, http.StatusOK)
	}
}

func (api *Controller) UpdateBook(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (api *Controller) DeleteBook(w http.ResponseWriter, r *http.Request) {
	db, ok := r.Context().Value(contextKrudDatabaser{}).(Databaser)
	if !ok {
		WriteJson(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}

	authorID, err := GetIntFromRequest(r, "authorID")
	if err != nil {
		WriteJsonError(w, err, http.StatusInternalServerError)
		return
	}

	bookID, err := GetIntFromRequest(r, "bookID")
	if err != nil {
		WriteJsonError(w, err, http.StatusInternalServerError)
	}

	err = db.DeleteBook(r.Context(), int64(authorID), int64(bookID))
	if err != nil {
		if errors.Is(err, ErrDoesNotExist) {
			WriteJsonError(w, err, http.StatusNotFound)
			return
		}
		WriteJsonError(w, err, http.StatusInternalServerError)
		return
	}
	// Should we return repr of deleted resource?
	w.WriteHeader(http.StatusNoContent)
}

func (api *Controller) Events(w http.ResponseWriter, r *http.Request) {

	db, ok := r.Context().Value(contextKrudDatabaser{}).(Databaser)
	if !ok {
		WriteJson(w, errors.New("internal error"), http.StatusInternalServerError)
		return
	}

	filters := []Filter{}
	queries := struct {
		Before time.Time `json:"before"`
		After  time.Time `json:"after"`
	}{}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&queries)
	if err == io.EOF {
		// Indicates no body, i.e. no filters, skip ahead.
	} else if err != nil {
		WriteJsonError(w, err, http.StatusBadRequest)
		return
	} else {
		if !queries.Before.IsZero() {
			filters = append(filters, EventsBefore(queries.Before))
		}
		if !queries.After.IsZero() {
			filters = append(filters, EventsAfter(queries.After))
		}
	}

	events, err := db.QueryEvents(r.Context(), filters...)
	if err != nil {
		WriteJsonError(w, err, http.StatusInternalServerError)
	}
	WriteJson(w, events, http.StatusOK)
}
