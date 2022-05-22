package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v4/stdlib"
	log "github.com/sirupsen/logrus"

	"github.com/vikblom/krud"
)

func main() {
	// Inputs
	addr := flag.String("addr", ":8080", "HTTP server addr")
	url := flag.String("url", "", "postgresql URL")
	loglevel := flag.String("loglevel", "info", "log verbosity")
	flag.Parse()
	if *url == "" {
		flag.Usage()
		return
	}
	logger := log.New()
	lvl, err := log.ParseLevel(*loglevel)
	if err != nil {
		println(err.Error())
		return
	}
	logger.SetLevel(lvl)

	// Krud
	db, err := sql.Open("pgx", *url)
	if err != nil {
		logger.Fatalf("open DB: %v", err)
	}
	defer db.Close()
	// Wrap actual db to match endpoint controller API.
	dial := func(ctx context.Context, user string) (krud.Databaser, error) {
		logger.Debug("dialing DB conn for user: %s", user)
		return krud.NewAuditDB(ctx, db, user)
	}

	r := mux.NewRouter()

	r.HandleFunc("/", HandleHello)

	// "Proper" endpoint w/ user checking.
	sr := r.PathPrefix("/api").Subrouter()
	_ = krud.NewController(logger, sr, krud.DialFunc(dial))

	logger.Infof("Serving HTTP at: %s", *addr)
	logger.Fatal(http.ListenAndServe(*addr, r))
}

func HandleHello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "HELLO\n")
}
