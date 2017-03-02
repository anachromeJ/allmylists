package main
import (
  "log"
  "fmt"
  "net/http"
  "os"
	"io/ioutil"
	"database/sql"
	_ "github.com/lib/pq"
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
		passhash string
	)

	for rows.Next() {
		err := rows.Scan(&email, &firstname, &lastname, &passhash)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(email, firstname, lastname, passhash)
	}

  fmt.Fprintln(w, "OK")
}

func main() {
  addr, err := determineListenAddress()
  if err != nil {
    log.Fatal(err)
  }

  http.HandleFunc("/", mainHandler)
	http.HandleFunc("/db", dbHandler)

  log.Printf("Listening on %s...\n", addr)
  if err := http.ListenAndServe(addr, nil); err != nil {
    panic(err)
  }
}
