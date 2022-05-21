package krud

// NOTES:
// An ORM would have got started quicker.
// Could typ xID int64.
// Bake ID into structs vs. keep outside.
// Code and tests are very boilerplate-y.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	// Register "pgx" driver in database/sql

	_ "github.com/jackc/pgx/v4/stdlib"
)

// FIXME: Maybe better as a "db handle", un-pointer method receiver?
type AuditDB struct {
	db   *sql.DB
	user string
}

const (
	AUDIT_OP_CREATE = "CREATE"
	AUDIT_OP_READ   = "READ"
	AUDIT_OP_UPDATE = "UPDATE"
	AUDIT_OP_DELETE = "DELETE"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrDoesNotExist = errors.New("object not found")

func NewAuditDB(ctx context.Context, db *sql.DB, user string) (*AuditDB, error) {

	ok, err := authorize(ctx, db, user)
	if err != nil {
		return nil, fmt.Errorf("authorization: %w", err)
	}
	if ok {
		return &AuditDB{db: db, user: user}, nil
	} else {
		return nil, ErrUnauthorized
	}
}

// authorize checks if user is allowed to use db.
func authorize(ctx context.Context, db *sql.DB, user string) (ok bool, err error) {

	// Not yet an AuditDB so cannot use wrapInTransaction.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("create transaction: %w", err)
	}
	defer func() {
		err2 := tx.Rollback()
		// Don't shadow earlier error. Ignore "already rolled back/comitted".
		if err == nil && err2 != nil && !errors.Is(err2, sql.ErrTxDone) {
			err = err2
		}
	}()

	_, err = db.ExecContext(ctx,
		`INSERT INTO events (username, obj_type, operation, ts)
         VALUES ($1, $2, $3, NOW())`,
		user,
		"auth",
		AUDIT_OP_READ)
	if err != nil {
		return false, fmt.Errorf("insert event: %w", err)
	}

	rows, err := db.QueryContext(ctx, "SELECT name FROM users")
	if err != nil {
		return false, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	// ok is zero-valued to false
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, fmt.Errorf("scanning row: %w", err)
		}
		if user == name {
			ok = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("going over rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit transaction: %w", err)
	}

	return ok, nil
}

// wrapInTransaction create a transaction for action.
// DB operations in action either happens all together, or not all all.
// action can be a closure to escape side-effects.
// NOTE: Possibly too "magical".
func (adb *AuditDB) wrapInTransaction(ctx context.Context, action func(tx *sql.Tx) error) error {

	// A transaction must end with a call to Commit or Rollback.
	tx, err := adb.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	err = action(tx)
	if err != nil {
		rbErr := tx.Rollback()
		if rbErr != nil {
			return fmt.Errorf("rollback failed because '%v' after err: %w", rbErr, err)
		}
		return err
	}
	return tx.Commit()
}

func (adb *AuditDB) AddAuthor(ctx context.Context, author Author) (id int64, err error) {

	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`INSERT INTO authors (name, date_of_birth)
             VALUES($1,$2)
             RETURNING id`,
			author.Name,
			author.DateOfBirth)
		if row.Err() != nil {
			return fmt.Errorf("insert author: %w", err)
		}
		// Put value in output.
		err = row.Scan(&id)
		if err != nil {
			return fmt.Errorf("scanning id: %w", err)
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, obj_id, operation, ts)
             VALUES ($1, $2, $3, $4, NOW())`,
			adb.user,
			"authors",
			id,
			AUDIT_OP_CREATE)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		return nil
	})
	if err != nil {
		return -1, fmt.Errorf("transaction: %w", err)
	}

	return id, err
}

func (adb *AuditDB) GetAuthor(ctx context.Context, id int64) (author *Author, err error) {

	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, obj_id, operation, ts)
             VALUES ($1, $2, $3, $4, NOW())`,
			adb.user,
			"authors",
			id,
			AUDIT_OP_READ)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}

		row := tx.QueryRowContext(ctx,
			`SELECT id, name, date_of_birth
             FROM authors
             WHERE id=$1`,
			id)
		if row.Err() != nil {
			return fmt.Errorf("select authors: %w", row.Err())
		}

		author = new(Author)
		if err := row.Scan(&author.ID, &author.Name, &author.DateOfBirth); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrDoesNotExist
			}
			return fmt.Errorf("scanning row: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction: %w", err)
	}

	return author, nil
}

func (adb *AuditDB) UpdateAuthor(ctx context.Context, author Author) (err error) {
	// TOOD: Only update given values. Maybe map[id]interface{}.

	var n int64
	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, obj_id, operation, ts)
             VALUES ($1, $2, $3, $4, NOW())`,
			adb.user,
			"authors",
			author.ID,
			AUDIT_OP_UPDATE)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}

		res, err := tx.ExecContext(ctx,
			`UPDATE authors
             SET name=$2, date_of_birth=$3
             WHERE id=$1`,
			author.ID,
			author.Name,
			author.DateOfBirth,
		)
		if err != nil {
			return fmt.Errorf("update author: %w", err)
		}

		n, err = res.RowsAffected()
		if err != nil {
			return fmt.Errorf("affected rows: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("transaction: %w", err)
	}

	if n == 0 {
		return ErrDoesNotExist
	}
	return nil
}

func (adb *AuditDB) AllAuthors(ctx context.Context) (authors []Author, err error) {

	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, operation, ts)
             VALUES ($1,$2,$3,NOW())`,
			adb.user,
			"authors",
			AUDIT_OP_READ)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}

		rows, err := tx.QueryContext(ctx, "SELECT id, name, date_of_birth FROM authors")
		if err != nil {
			return fmt.Errorf("select authors: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id int64
			var name string
			var bday Date
			if err := rows.Scan(&id, &name, &bday); err != nil {
				return fmt.Errorf("scanning row: %w", err)
			}
			authors = append(authors, Author{ID: id, Name: name, DateOfBirth: bday})
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("going over rows: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction: %w", err)
	}

	return authors, nil
}

func (adb *AuditDB) DeleteAuthor(ctx context.Context, id int64) (err error) {

	var n int64
	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, obj_id, operation, ts)
             VALUES ($1, $2, $3, $4, NOW())`,
			adb.user,
			"authors",
			id,
			AUDIT_OP_DELETE)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}

		res, err := tx.ExecContext(ctx, `DELETE FROM authors WHERE id=$1`, id)
		if err != nil {
			return fmt.Errorf("delete author: %w", err)
		}

		n, err = res.RowsAffected()
		if err != nil {
			return fmt.Errorf("affected rows: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("transaction: %w", err)
	}

	if n == 0 {
		return ErrDoesNotExist
	}
	return nil
}

func (adb *AuditDB) AddBook(ctx context.Context, author int64, book Book) (id int64, err error) {

	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`INSERT INTO books (author_id, title, published)
             VALUES($1, $2, $3)
             RETURNING id`,
			author,
			book.Title,
			book.Published)
		if row.Err() != nil {
			return fmt.Errorf("insert book: %w", err)
		}
		// Put value in output.
		err = row.Scan(&id)
		if err != nil {
			return fmt.Errorf("scanning id: %w", err)
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, obj_id, operation, ts)
             VALUES ($1, $2, $3, $4, NOW())`,
			adb.user,
			"books",
			id,
			AUDIT_OP_CREATE)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		return nil
	})
	if err != nil {
		return -1, fmt.Errorf("transaction: %w", err)
	}

	return id, nil
}

func (adb *AuditDB) GetBook(ctx context.Context, authorID, bookID int64) (book *Book, err error) {

	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, obj_id, operation, ts)
             VALUES ($1, $2, $3, $4, NOW())`,
			adb.user,
			"books",
			bookID,
			AUDIT_OP_READ)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}

		row := tx.QueryRowContext(ctx,
			`SELECT id, title, published
             FROM books
             WHERE id=$1 AND author_id=$2`,
			bookID, authorID)
		if row.Err() != nil {
			return fmt.Errorf("select books: %w", row.Err())
		}

		book = new(Book)
		if err := row.Scan(&book.ID, &book.Title, &book.Published); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrDoesNotExist
			}
			return fmt.Errorf("scanning row: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction: %w", err)
	}

	return book, nil
}

func (adb *AuditDB) UpdateBook(ctx context.Context, authorID int64, book Book) (err error) {
	// TOOD: Only update given values. Maybe map[id]interface{}.

	var n int64
	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, obj_id, operation, ts)
             VALUES ($1, $2, $3, $4, NOW())`,
			adb.user,
			"books",
			book.ID,
			AUDIT_OP_UPDATE)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}

		res, err := tx.ExecContext(ctx,
			`UPDATE books
             SET title=$2, published=$3
             WHERE id=$1`,
			book.ID,
			book.Title,
			book.Published,
		)
		if err != nil {
			return fmt.Errorf("update book: %w", err)
		}

		n, err = res.RowsAffected()
		if err != nil {
			return fmt.Errorf("affected rows: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("transaction: %w", err)
	}

	if n == 0 {
		return ErrDoesNotExist
	}
	return nil
}

func (adb *AuditDB) AllBooks(ctx context.Context) (books []Book, err error) {

	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, operation, ts)
             VALUES ($1,$2,$3,NOW())`,
			adb.user,
			"books",
			AUDIT_OP_READ)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}

		rows, err := tx.QueryContext(ctx, "SELECT id, title, published FROM books")
		if err != nil {
			return fmt.Errorf("select books: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var b Book
			if err := rows.Scan(&b.ID, &b.Title, &b.Published); err != nil {
				return fmt.Errorf("scanning row: %w", err)
			}
			books = append(books, b)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("going over rows: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction: %w", err)
	}

	return books, nil
}

func (adb *AuditDB) DeleteBook(ctx context.Context, authorID, bookID int64) (err error) {

	var n int64
	err = adb.wrapInTransaction(ctx, func(tx *sql.Tx) error {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO events (username, obj_type, obj_id, operation, ts)
             VALUES ($1, $2, $3, $4, NOW())`,
			adb.user,
			"books",
			bookID,
			AUDIT_OP_DELETE)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}

		res, err := tx.ExecContext(ctx,
			`DELETE FROM books WHERE id=$1 AND author_id=$2`,
			bookID,
			authorID)
		if err != nil {
			return fmt.Errorf("delete books: %w", err)
		}

		n, err = res.RowsAffected()
		if err != nil {
			return fmt.Errorf("affected rows: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("transaction: %w", err)
	}

	if n == 0 {
		return ErrDoesNotExist
	}
	return nil
}

type Event struct {
	When      time.Time
	User      string
	Operation string
	Type      string
	ID        *int64 // Can be NULL.
}

// Filter is an option-like type that lets outside callers specify
// which events they are interested in, but the implementation of filtering
// out such events is hidden.
type Filter func(*whereFilter)

// whereFilter sets up for a 'WHERE lhs=rhs' to add to a SQL query.
type whereFilter struct {
	lhs []string
	rhs []interface{}
}

func EventsAfter(t time.Time) Filter {
	return func(f *whereFilter) {
		(*f).lhs = append((*f).lhs, fmt.Sprintf("ts > $%d::timestamp", len(f.lhs)+1))
		(*f).rhs = append((*f).rhs, t.UTC())
	}
}

func EventsBefore(t time.Time) Filter {
	return func(f *whereFilter) {
		(*f).lhs = append((*f).lhs, fmt.Sprintf("ts < $%d", len(f.lhs)+1))
		(*f).rhs = append((*f).rhs, t.UTC())
	}
}

// TODO: Unit test whereFilter.
// TODO: More filters.

func (wf *whereFilter) where() (string, []interface{}) {
	if len(wf.lhs) == 0 {
		return "", nil
	}

	var b strings.Builder
	fmt.Fprint(&b, "WHERE ")
	for i, left := range wf.lhs {
		if i != 0 {
			fmt.Fprint(&b, " AND ")
		}
		fmt.Fprint(&b, left)
	}
	return b.String(), wf.rhs
}

func (adb *AuditDB) QueryEvents(ctx context.Context, filters ...Filter) (events []Event, err error) {
	// Note that querying events does not create a new event.

	wfs := whereFilter{}
	for _, f := range filters {
		f(&wfs)
	}

	where, args := wfs.where()
	//fmt.Printf("%#v    %v\n", where, args)
	rows, err := adb.db.QueryContext(ctx,
		`SELECT ts, username, operation, obj_type, obj_id
         FROM events `+where,
		args...)
	if err != nil {
		return nil, fmt.Errorf("select events: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.When, &e.User, &e.Operation, &e.Type, &e.ID); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("going over rows: %w", err)
	}

	return events, nil
}
