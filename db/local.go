package db

import (
	"database/sql"
	"jpm/model"

	_ "github.com/tursodatabase/turso-go"
)

type LocalDB struct {
	Connection *sql.DB
}

func NewLocalDB() LocalDB {
	conn, _ := sql.Open("turso", "jpm.db")
	return LocalDB{
		Connection: conn,
	}
}

func (ldb *LocalDB) CreateInstallations() error {
	_, err := ldb.Connection.Exec(`
CREATE TABLE installed (
	name VARCHAR(100) PRIMARY KEY,
	version VARCHAR(20),
	sys_path VARCHAR(100),
	location VARCHAR(100)
);
`)
	return err
}

func (ldb *LocalDB) GetAll() []model.Installed {
	stmt, _ := ldb.Connection.Prepare("SELECT * FROM installed")
	defer stmt.Close()
	var all []model.Installed

	rows, _ := stmt.Query()
	for rows.Next() {
		var ins model.Installed
		_ = rows.Scan(&ins.Name, &ins.Version, &ins.SysPath, &ins.Location)
		all = append(all, ins)
	}
	return all
}

func (ldb *LocalDB) GetAllForList() []model.Installed {
	stmt, _ := ldb.Connection.Prepare("SELECT name, version FROM installed")
	defer stmt.Close()
	var all []model.Installed

	rows, _ := stmt.Query()
	for rows.Next() {
		var ins model.Installed
		_ = rows.Scan(&ins.Name, &ins.Version)
		all = append(all, ins)
	}
	return all
}

func (ldb *LocalDB) GetCount() int {
	stmt, _ := ldb.Connection.Prepare("SELECT COUNT(name) AS count FROM installed LIMIT 1")
	defer stmt.Close()
	count := 0
	_ = stmt.QueryRow().Scan(&count)
	return count
}

func (ldb *LocalDB) InsertInstallation(ins *model.Installed) error {
	_, err := ldb.Connection.Exec("INSERT INTO installed (name, version, sys_path, location) VALUES (?, ?, ?, ?)", ins.Name, ins.Version, ins.SysPath, ins.Location)
	return err
}

/*
	func (ldb *LocalDB) DefaultData() {
		_, err := ldb.Connection.Exec("INSERT INTO releases (name, version, binary_url) VALUES (?, ?, ?)", "jyntaxe", "0.0.0", "github/jpr/jyntaxe")
		if err != nil {
			panic(err)
		}
	}
*/
func (ldb *LocalDB) Close() {
	ldb.Connection.Close()
}
