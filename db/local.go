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
CREATE TABLE IF NOT EXISTS installed (
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

func (ldb *LocalDB) GetByName(name string) (*model.Installed, error) {
	stmt, err := ldb.Connection.Prepare("SELECT name, version, sys_path, location FROM installed WHERE name = ? LIMIT 1")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var ins model.Installed
	err = stmt.QueryRow(name).Scan(&ins.Name, &ins.Version, &ins.SysPath, &ins.Location)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ins, nil
}

func (ldb *LocalDB) GetCount() int {
	stmt, _ := ldb.Connection.Prepare("SELECT COUNT(name) AS count FROM installed LIMIT 1")
	defer stmt.Close()
	count := 0
	_ = stmt.QueryRow().Scan(&count)
	return count
}

func (ldb *LocalDB) InsertInstallation(ins *model.Installed) error {
	_, err := ldb.Connection.Exec("INSERT INTO installed (name, version, sys_path, location) VALUES (?, ?, ?, ?)",
		ins.Name, ins.Version, ins.SysPath, ins.Location)
	return err
}

func (ldb *LocalDB) UpdateInstallation(ins *model.Installed) error {
	_, err := ldb.Connection.Exec("UPDATE installed SET version = ?, sys_path = ?, location = ? WHERE name = ?",
		ins.Version, ins.SysPath, ins.Location, ins.Name)
	return err
}

func (ldb *LocalDB) DeleteInstallation(name string) error {
	_, err := ldb.Connection.Exec("DELETE FROM installed WHERE name = ?", name)
	return err
}

func (ldb *LocalDB) Close() {
	ldb.Connection.Close()
}
