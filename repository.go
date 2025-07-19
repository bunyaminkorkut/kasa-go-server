package main

import (
	"database/sql"
)

type UserRepository struct {
	DB *sql.DB
}

func (repo *UserRepository) CreateUser(id, username, email, hashedPassword string) error {
	_, err := repo.DB.Exec("INSERT INTO users (id, username, email, password) VALUES (?, ?, ?, ?)", id, username, email, hashedPassword)
	return err
}
