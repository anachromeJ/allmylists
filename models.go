package main

type User struct {
	Id        int
	Email     string
	FirstName string
	LastName  string
}

type List struct {
	Id       string
	Source   string
	RootItem string
	Owner    int
	Created  string
}

type Item struct {
	ID        string
	Notes     string
	DateTime1 string
	DateTime2 string
	ParentId  string
	Title     string
	Checked   string
	Created   string
}

type NestedItem struct {
	ID        string
	Notes     string
	DateTime1 string
	DateTime2 string
	Children  []string
	Title     string
	Checked   string
	Created   string
}
