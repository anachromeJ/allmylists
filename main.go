package main
import (
  "log"
  "fmt"
	"time"
  "os"
	"strings"
	"encoding/json"
  "net/http"
	"io/ioutil"
	"database/sql"
	_ "github.com/lib/pq"
	"github.com/gorilla/mux"
	"github.com/goware/emailx"
)

var db *sql.DB

const MAX_NUM_LISTS = 2048 // limit on a user's owned lists
const MAX_NUM_ITEMS = 65536 // limit on a user's owned items (should there be one? well yes. but how much?)

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
	Created string
}

type Item struct {
	Id string
	Notes string
	DateTime1 string
	DateTime2 string
	ParentId string
	Title string
	Checked string
	Created string
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
	log.Println(filePath)

	var bytes []byte
	var err error
	if filePath == "/" {
		bytes, err = ioutil.ReadFile("frontend/src/index.html")
	} else {
		bytes, err = ioutil.ReadFile("frontend/src" + filePath)
	}

	// TODO: error code
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
		firstName sql.NullString
		lastName sql.NullString
	)

	for rows.Next() {
		err := rows.Scan(&email, &firstName, &lastName)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(email, firstName.String, lastName.String)
	}

  fmt.Fprintln(w, "OK")
}

func newUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	if r.Body == nil {
		http.Error(w, "Please send a request body", 400)
		return
	}

	var u User
	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	err = emailx.Validate(u.Email)
	if err != nil {
		http.Error(w, "Invalid email", 400)
		return
	}
	fmt.Printf("%v\n", u)

	query := fmt.Sprintf("INSERT INTO users VALUES ('%s', '%s', '%s')", u.Email, u.FirstName, u.LastName)
	_, err = db.Query(query)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "That email is already registered", 400)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}

	query = fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", u.Email)
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defer rows.Close()

	if !(rows.Next()) {
		http.Error(w, "User does not exist", 500)
		return
	}

	var (
		email string
		firstName sql.NullString
		lastName sql.NullString
		id int
	)
	err = rows.Scan(&email, &firstName, &lastName, &id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	user := User{id, email, firstName.String, lastName.String}
	json.NewEncoder(w).Encode(user)
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userId := vars["userId"]
	query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", userId)
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defer rows.Close()

	if !(rows.Next()) {
		http.Error(w, "User not found", 404)
		return
	}

	var (
		email string
		firstName sql.NullString
		lastName sql.NullString
		id int
	)
	err = rows.Scan(&email, &firstName, &lastName, &id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	user := User{id, email, firstName.String, lastName.String}
	json.NewEncoder(w).Encode(user)
}

func userListsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userId := vars["userId"]
	query := fmt.Sprintf("SELECT * FROM lists WHERE owner = %s", userId)
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defer rows.Close()

	// TODO: this'll throw an error if max is exceeded
	lists := make([]List, 0, MAX_NUM_LISTS)
	for rows.Next() {
		var (
			id string
			source string
			root string
			owner int
			created string
		)
		err = rows.Scan(&id, &source, &root, &owner, &created)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		
		lists = append(lists, List{id, source, root, owner, created})
	}

	if r.Method == "PUT" {
		newLists := make([]List, 0, MAX_NUM_LISTS)
		err := json.NewDecoder(r.Body).Decode(&newLists)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		combined := make(map[string]List)
		for _, list := range lists {
			combined[list.Id] = list
		}

		for _, list := range newLists {
			// TODO: do a timestamp check here
			combined[list.Id] = list
		}

		lists = make([]List, 0, MAX_NUM_LISTS)
		for _, value := range combined {
			lists = append(lists, value)
		}
	}

	json.NewEncoder(w).Encode(lists)
}

func userItemsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userId := vars["userId"]
	query := fmt.Sprintf(
		`WITH RECURSIVE t(id, notes, datetime1, datetime2, parent_id, title, checked, created) AS (
         SELECT items.*
         FROM items
         JOIN lists
         ON items.id = lists.root_item
         WHERE lists.owner = %s
       UNION ALL
         SELECT items.*
         FROM items JOIN t
         ON items.parent_id = t.id
     )
     SELECT * FROM t;`,
		userId)

	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	items := make([]Item, 0, MAX_NUM_ITEMS)
	for rows.Next() {
		var (
			id string
			notes sql.NullString
			dt1 sql.NullString
			dt2 sql.NullString
			parentId sql.NullString
			title string
			checked string
			created string
		)

		err = rows.Scan(&id, &notes, &dt1, &dt2, &parentId,
			&title, &checked, &created)
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
			created,
		}
		items = append(items, item)
	}

	if r.Method == "PUT" {
		newItems := make([]Item, 0, MAX_NUM_ITEMS)
		err := json.NewDecoder(r.Body).Decode(&newItems)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		combined := make(map[string]Item)
		for _, item := range items {
			combined[item.Id] = item
		}

		for _, item := range newItems {
			// TODO: do a timestamp check here
			combined[item.Id] = item
		}

		items = make([]Item, 0, MAX_NUM_ITEMS)
		for _, value := range combined {
			items = append(items, value)
		}
	}

	json.NewEncoder(w).Encode(items)
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
		created string
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
		created,
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
		fmt.Fprintf(w, "404: list not found")
		return
	}

	var (
		id string
		source string
		root string
		owner int
		created string
	)

	err = rows.Scan(&id, &source, &root, &owner, &created)
	if err != nil {
		// TODO: 500
		log.Println(err)
		return
	}

	list := List{id, source, root, owner, created}
	json.NewEncoder(w).Encode(list)	
}

func listItemsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	listId := vars["listId"]
	query := fmt.Sprintf(
		`WITH RECURSIVE t(id, notes, datetime1, datetime2, parent_id, title, checked, created) AS (
         SELECT items.*
         FROM items
         JOIN lists
         ON items.id = lists.root_item
         WHERE lists.id = '%s'
       UNION ALL
         SELECT items.*
         FROM items JOIN t
         ON items.parent_id = t.id
     )
     SELECT * FROM t;`,
		listId)

	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	items := make([]Item, 0, MAX_NUM_ITEMS)
	for rows.Next() {
		var (
			id string
			notes sql.NullString
			dt1 sql.NullString
			dt2 sql.NullString
			parentId sql.NullString
			title string
			checked string
			created string
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
			created,
		}
		items = append(items, item)
	}

	json.NewEncoder(w).Encode(items)
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
	r.HandleFunc("/users", newUserHandler).
		Methods("POST")
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

	srv := &http.Server{
		Handler:      r,
		Addr:         addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
