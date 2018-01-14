package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/goware/emailx"
)

func dbHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("select * from users limit 1")
	if err != nil {
		log.Println(err)
	}

	defer rows.Close()

	var (
		email     string
		firstName sql.NullString
		lastName  sql.NullString
	)

	i := 0
	for rows.Next() {
		err := rows.Scan(&email, &firstName, &lastName)
		if err != nil {
			log.Println(err)
		}
		i++
	}
	fmt.Fprintln(w, "OK")
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, "Request body is empty", 400)
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
	_, err = db.Exec(query)
	if err != nil {
		isError := true
		if strings.Contains(err.Error(), "duplicate key") {
			// user already registered -- just return user info
			isError = false
		}

		if isError {
			http.Error(w, err.Error(), 500)
			return
		}
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
		email     string
		firstName sql.NullString
		lastName  sql.NullString
		id        int
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
		email     string
		firstName sql.NullString
		lastName  sql.NullString
		id        int
	)
	err = rows.Scan(&email, &firstName, &lastName, &id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	user := User{id, email, firstName.String, lastName.String}
	json.NewEncoder(w).Encode(user)
}

func syncUserLists(lists []List, newLists []List) ([]List, int, error) {
	combined := make(map[string]List)
	for _, list := range lists {
		combined[list.Id] = list
	}

	inserts := make([]List, 0, len(newLists))
	updates := make([]List, 0, len(newLists))
	for _, list := range newLists {
		// TODO: do a timestamp check here
		if list0, ok := combined[list.Id]; ok && list0 != list {
			updates = append(updates, list)
		} else {
			inserts = append(inserts, list)
		}
		combined[list.Id] = list
	}

	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		tx.Rollback()
		return nil, 500, err
	}

	insertStmt, err := db.Prepare(`INSERT INTO lists (id, source, root_item, owner) VALUES ($1, $2, $3, $4)`)
	if err != nil {
		log.Println(err)
		tx.Rollback()
		return nil, 500, err
	}

	updateStmt, err := db.Prepare(`UPDATE lists SET source = ($1), root_item = ($2), owner = ($3) WHERE id = ($4)`)
	if err != nil {
		log.Println(err)
		tx.Rollback()
		return nil, 500, err
	}

	for _, list := range inserts {
		fmt.Println("inserting list", list.Id, list.Source, list.RootItem, list.Owner)
		_, err = insertStmt.Exec(list.Id, list.Source, list.RootItem, list.Owner)
		if err != nil {
			log.Println(err)
			tx.Rollback()
			return nil, 500, err
		}
	}
	for _, list := range updates {
		_, err = updateStmt.Exec(list.Source, list.RootItem, list.Owner, list.Id)
		if err != nil {
			log.Println(err)
			tx.Rollback()
			return nil, 500, err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Println(err)
		tx.Rollback()
		return nil, 500, err
	}

	lists = make([]List, 0, MaxNumLists)
	for _, value := range combined {
		lists = append(lists, value)
	}

	return lists, 200, nil
}

func userListsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["userId"]
	query := fmt.Sprintf("SELECT * FROM lists WHERE owner = %s", userID)
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defer rows.Close()

	// TODO: this'll throw an error if max is exceeded
	lists := make([]List, 0, MaxNumLists)
	for rows.Next() {
		var (
			id      string
			source  string
			root    string
			owner   int
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
		newLists := make([]List, 0, MaxNumLists)
		err = json.NewDecoder(r.Body).Decode(&newLists)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		var code int
		lists, code, err = syncUserLists(lists, newLists)
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		fmt.Printf("synced lists for %s, total %d lists\n", userID, len(lists))
	}
	json.NewEncoder(w).Encode(lists)
}

func syncUserItems(items []Item, newItems []Item) ([]Item, int, error) {
	combined := make(map[string]Item)
	for _, item := range items {
		combined[item.ID] = item
	}

	inserts := make([]Item, 0, len(newItems))
	updates := make([]Item, 0, len(newItems))
	for _, item := range newItems {
		// TODO: do a timestamp check here
		if item0, ok := combined[item.ID]; ok && item0 != item {
			updates = append(updates, item)
		} else {
			inserts = append(inserts, item)
		}
		combined[item.ID] = item
	}

	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		tx.Rollback()
		return nil, 500, err
	}

	insertStmt, err := db.Prepare(`INSERT INTO items (id, notes, parent_id, title, checked) VALUES ($1, $2, $3, $4, $5)`)
	if err != nil {
		log.Println(err)
		tx.Rollback()
		return nil, 500, err
	}

	updateStmt, err := db.Prepare(`UPDATE items SET notes = ($1), parent_id = ($2), title = ($3), checked = ($4) WHERE id = ($5)`)
	if err != nil {
		log.Println(err)
		tx.Rollback()
		return nil, 500, err
	}

	for _, item := range inserts {
		fmt.Println("inserting item", item.ID, item.Notes, item.ParentId, item.Title, item.Checked)
		_, err = insertStmt.Exec(item.ID, item.Notes, item.ParentId, item.Title, item.Checked)
		if err != nil {
			log.Println(err)
			tx.Rollback()
			return nil, 500, err
		}
	}
	for _, item := range updates {
		_, err = updateStmt.Exec(item.Notes, item.ParentId, item.Title, item.Checked, item.ID)
		if err != nil {
			log.Println(err)
			tx.Rollback()
			return nil, 500, err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Println(err)
		tx.Rollback()
		return nil, 500, err
	}

	items = make([]Item, 0, MaxNumItems)
	for _, value := range combined {
		items = append(items, value)
	}

	return items, 200, nil
}

func denestItems(nestedItems []NestedItem) []Item {
	itemMap := make(map[string]*Item)
	for _, nestedItem := range nestedItems {
		item := Item{
			nestedItem.ID,
			nestedItem.Notes,
			nestedItem.DateTime1,
			nestedItem.DateTime2,
			"",
			nestedItem.Title,
			nestedItem.Checked,
			nestedItem.Created,
		}
		itemMap[nestedItem.ID] = &item
	}

	for _, nestedItem := range nestedItems {
		for _, id := range nestedItem.Children {
			itemMap[id].ParentId = nestedItem.ID
		}
	}

	items := make([]Item, 0, len(itemMap))
	for _, item := range itemMap {
		items = append(items, *item)
	}

	return items
}

func userItemsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["userId"]
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
		userID)

	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	items := make([]Item, 0, MaxNumItems)
	for rows.Next() {
		var (
			id       string
			notes    sql.NullString
			dt1      sql.NullString
			dt2      sql.NullString
			parentId sql.NullString
			title    string
			checked  string
			created  string
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
		nestedItems := make([]NestedItem, 0, MaxNumItems)
		err := json.NewDecoder(r.Body).Decode(&nestedItems)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		newItems := denestItems(nestedItems)
		fmt.Println(newItems)

		var code int
		items, code, err = syncUserItems(items, newItems)
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		fmt.Printf("synced items for %s, total %d items\n", userID, len(items))
	}
	json.NewEncoder(w).Encode(items)
}

// TODO: POST and PUT
func itemHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["itemId"]
	query := fmt.Sprintf("SELECT * FROM items WHERE id = '%s'", itemID)
	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	// TODO: return 404 code
	if !(rows.Next()) {
		http.Error(w, "Item not found", 404)
		return
	}

	var (
		id       string
		notes    sql.NullString
		dt1      sql.NullString
		dt2      sql.NullString
		parentID sql.NullString
		title    string
		checked  string
		created  string
	)

	err = rows.Scan(&id, &notes, &dt1, &dt2, &parentID, &title, &checked)
	if err != nil {
		log.Println(err)
		return
	}

	item := Item{
		id,
		notes.String,
		dt1.String,
		dt2.String,
		parentID.String,
		title,
		checked,
		created,
	}
	json.NewEncoder(w).Encode(item)
}

// TODO: POST and PUT
func listHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	listID := vars["listId"]
	query := fmt.Sprintf("SELECT * FROM lists WHERE id = '%s'", listID)
	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	if !(rows.Next()) {
		http.Error(w, "List not found", 404)
		return
	}

	var (
		id      string
		source  string
		root    string
		owner   int
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
	listID := vars["listId"]
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
		listID)

	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	items := make([]Item, 0, MaxNumItems)
	for rows.Next() {
		var (
			id       string
			notes    sql.NullString
			dt1      sql.NullString
			dt2      sql.NullString
			parentId sql.NullString
			title    string
			checked  string
			created  string
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
