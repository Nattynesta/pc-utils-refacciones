package main

import "database/sql"

func initSchema(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}
