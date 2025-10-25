package db

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

const (
	url   = "libsql://jpm-jasnrathore.aws-ap-south-1.turso.io"
	token = "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJhIjoicm8iLCJpYXQiOjE3NjEzNzM5OTcsImlkIjoiYWM1N2NmY2ItNTFmMi00MzU5LThjYTktODQzODFmOGY4MTdhIiwicmlkIjoiMzZjZGUxZjEtNjQyZS00ZGFkLTk5YWQtZGU0ZWYyZTkzNmIwIn0.5Wah3NkS6u1vz0yiZjuYLScGYGwGCZTxi2WLNhlIyy7P4wJLWuUDXl3gv3ja7jJ-bW_AZDdnuYSlt5tUS6WdCA"
)

type Metadata struct {
	Version      string
	Url          string
	Instructions string
}

type RemoteDB struct {
	Connection *sql.DB
}

func NewRemoteDB() RemoteDB {
	newUrl := fmt.Sprintf("%s?authToken=%s", url, token)
	conn, _ := sql.Open("libsql", newUrl)
	return RemoteDB{
		Connection: conn,
	}
}

func (ldb *RemoteDB) GetAll() {
	stmt, _ := ldb.Connection.Prepare("SELECT * FROM releases")
	defer stmt.Close()

	rows, _ := stmt.Query()
	for rows.Next() {
		var name string
		var binary_url string
		var version string
		var instructions string
		_ = rows.Scan(&name, &version, &binary_url, &instructions)
		fmt.Printf("%s, %s, %s\n", name, version, binary_url)
	}
}

func (rdb *RemoteDB) GetOne(name string) (*Metadata, error) {
	stmt, _ := rdb.Connection.Prepare("SELECT version, binary_url, instructions FROM releases WHERE name=? LIMIT 1")
	defer stmt.Close()

	rows, _ := stmt.Query(name)
	for rows.Next() {
		var binary_url string
		var version string
		var instructions string
		_ = rows.Scan(&version, &binary_url, &instructions)
		return &Metadata{version, binary_url, instructions}, nil
	}
	return nil, errors.New("Package Not Found")
}
func (rdb *RemoteDB) Close() {
	rdb.Connection.Close()
}
