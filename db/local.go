package db

import (
	"database/sql"
	"fmt"
	_ "github.com/tursodatabase/turso-go"
)

type LocalDB struct {
	Connection *sql.DB
}

func NewLocalDB() LocalDB {
	conn, _ := sql.Open("turso", "sqlite.db")
	return LocalDB{
		Connection: conn,
	}
}

func (ldb *LocalDB) CreateReleases() {
	_, err := ldb.Connection.Exec(`
CREATE TABLE releases (
	name VARCHAR(100) PRIMARY KEY,
	version VARCHAR(20),
	binary_url VARCHAR(100),
	instructions TEXT
)
`)
	if err != nil {
		panic(err)
	}
}
func (ldb *LocalDB) GetAll() {
	stmt, _ := ldb.Connection.Prepare("SELECT * FROM releases")
	defer stmt.Close()

	rows, _ := stmt.Query()
	for rows.Next() {
		var name string
		var binary_url string
		var version string
		var instructions string
		_ = rows.Scan(&name, &version, &binary_url, &instructions)
		fmt.Printf("%s, %s, %s\n%s\n", name, version, binary_url, instructions)
	}
}

func (ldb *LocalDB) DefaultData() {
	_, err := ldb.Connection.Exec("INSERT INTO releases (name, version, binary_url) VALUES (?, ?, ?)", "jyntaxe", "0.0.0", "github/jpr/jyntaxe")
	if err != nil {
		panic(err)
	}
}

func (ldb *LocalDB) Close() {
	ldb.Connection.Close()
}
