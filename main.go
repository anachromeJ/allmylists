package main
import (
  "log"
  "fmt"
	"time"
  "os"
	"encoding/json"
  "net/http"
	"io/ioutil"
	"database/sql"
	_ "github.com/lib/pq"
	"github.com/gorilla/mux"
)

var db *sql.DB

const MAX_NUM_LISTS = 256 // limit on a user's owned lists

type User struct {
	Id int
	Email string
	FirstName string
	LastName string
}

type List struct {
	Id string
	Source string
	RootItem string
	Owner int
}

type Item struct {
	Id string
	Notes string
	DateTime1 string
	DateTime2 string
	ParentId string
	Title string
	Checked string
}

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
	rows, err := db.Query("select * from users")
	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	var (
		email string
		firstname sql.NullString
		lastname sql.NullString
	)

	for rows.Next() {
		err := rows.Scan(&email, &firstname, &lastname)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(email, firstname.String, lastname.String)
	}

  fmt.Fprintln(w, "OK")
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userId := vars["userId"]
	query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", userId)
	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	// TODO: return 404 code
	if !(rows.Next()) {
		fmt.Fprintf(w, "user not found")
		return
	}

	var (
		email string
		firstname sql.NullString
		lastname sql.NullString
		id int
	)
	err = rows.Scan(&email, &firstname, &lastname, &id)
	if err != nil {
		log.Println(err)
		return
	}

	user := User{id, email, firstname.String, lastname.String}
	json.NewEncoder(w).Encode(user)
}

func userListsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userId := vars["userId"]
	query := fmt.Sprintf("SELECT * FROM lists WHERE owner = %s", userId)
	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	lists := make([]List, 0, MAX_NUM_LISTS)
	for rows.Next() {
		var (
			id string
			source string
			root string
			owner int
		)
		err = rows.Scan(&id, &source, &root, &owner)
		if err != nil {
			log.Println(err)
			return
		}
		
		lists = append(lists, List{id, source, root, owner})
	}

	json.NewEncoder(w).Encode(lists)
}

// TODO: POST and PUT
func itemHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemId := vars["itemId"]
	query := fmt.Sprintf("SELECT * FROM items WHERE id = '%s'", itemId)
	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	// TODO: return 404 code
	if !(rows.Next()) {
		fmt.Fprintf(w, "404: item not found")
		return
	}

	var (
		id string
		notes sql.NullString
		dt1 sql.NullString
		dt2 sql.NullString
		parentId sql.NullString
		title string
		checked string
	)

	err = rows.Scan(&id, &notes, &dt1, &dt2, &parentId, &title, &checked)
	if err != nil {
		log.Println(err)
		return
	}

	item := Item{
		id,
		notes.String,
		dt1.String,
		dt2.String,
		parentId.String,
		title,
		checked,
	}
	json.NewEncoder(w).Encode(item)
}

// TODO: POST and PUT
func listHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	listId := vars["listId"]
	query := fmt.Sprintf("SELECT * FROM lists WHERE id = '%s'", listId)
	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	// TODO: return 404 code
	if !(rows.Next()) {
		fmt.Fprintf(w, "404: item not found")
		return
	}

	var (
		id string
		source string
		root string
		owner int
	)

	err = rows.Scan(&id, &source, &root, &owner)
	if err != nil {
		// TODO: 500
		log.Println(err)
		return
	}

	list := List{id, source, root, owner}
	json.NewEncoder(w).Encode(list)	
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
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/db", dbHandler)
	r.HandleFunc("/users/{userId}", userHandler).
		Methods("GET", "POST")
	r.HandleFunc("/users/{userId}/lists", userListsHandler)
	r.HandleFunc("/items/{itemId}", itemHandler).
		Methods("GET", "POST", "PUT")
	r.HandleFunc("/lists/{listId}", listHandler).
		Methods("GET", "POST", "PUT")
	
	srv := &http.Server{
		Handler:      r,
		Addr:         addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
