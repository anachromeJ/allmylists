package main
import (
  "log"
  "fmt"
	"time"
  "net/http"
  "os"
	"io/ioutil"
	"database/sql"
	_ "github.com/lib/pq"
	"github.com/gorilla/mux"
)

func determineListenAddress() (string, error) {
  port := os.Getenv("PORT")
  if port == "" {
    return "", fmt.Errorf("$PORT not set")
  }
  return ":" + port, nil
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.String()

	var bytes []byte
	var err error
	if filePath == "/" {
		bytes, err = ioutil.ReadFile("frontend/src/index.html")
	} else {
		bytes, err = ioutil.ReadFile("frontend/src" + filePath)
	}

	// TODO: create a new stderr logger
	if err != nil {
		log.Println(err)
	}

	_, err = w.Write(bytes)
	if err != nil {
		log.Println(err)
	}
}

func dbHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query("select * from users")
	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	var (
		email string
		firstname string
		lastname string
	)

	for rows.Next() {
		err := rows.Scan(&email, &firstname, &lastname)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(email, firstname, lastname)
	}

  fmt.Fprintln(w, "OK")
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "user")
}

func userListsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "userLists")
}

func taskHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "tasks")
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "lists")
}

func main() {
  addr, err := determineListenAddress()
  if err != nil {
    log.Fatal(err)
  }

	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/db", dbHandler)
	r.HandleFunc("/users/{id}", userHandler)
	r.HandleFunc("/users/{id}/lists", userListsHandler)
	r.HandleFunc("/tasks/{id}", taskHandler)
	r.HandleFunc("/lists/{id}", listHandler)
	
	srv := &http.Server{
		Handler:      r,
		Addr:         addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
