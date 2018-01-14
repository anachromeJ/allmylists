package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
)

var db *sql.DB

// limit on a user's owned lists
const MaxNumLists = 2048

// limit on a user's owned items (should there be one? well yes. but how much?)
const MaxNumItems = 65536

func determineListenAddress() (string, error) {
	port := os.Getenv("PORT")
	if port == "" {
		return "", fmt.Errorf("$PORT not set")
	}
	return ":" + port, nil
}

func main() {
	addr, err := determineListenAddress()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("opening connection to database")
	db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/db", dbHandler)
	r.HandleFunc("/users", loginHandler).
		Methods("POST", "PUT")
	r.HandleFunc("/users/{userId}", userHandler).
		Methods("GET")
	r.HandleFunc("/users/{userId}/lists", userListsHandler).
		Methods("GET", "PUT")
	r.HandleFunc("/users/{userId}/items", userItemsHandler).
		Methods("GET", "PUT")
	r.HandleFunc("/items/{itemId}", itemHandler).
		Methods("GET", "POST", "PUT")
	r.HandleFunc("/lists/{listId}", listHandler).
		Methods("GET", "POST", "PUT")
	r.HandleFunc("/lists/{listId}/items", listItemsHandler).
		Methods("GET")
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./frontend/src/"))).
		Methods("GET")

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT"},
		AllowCredentials: true,
	})
	handler := c.Handler(r)

	srv := &http.Server{
		Handler: handler,
		Addr:    addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
