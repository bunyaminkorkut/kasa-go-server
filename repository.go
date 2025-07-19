package main

import (
	"database/sql"
	"log"
)

type UserRepository struct {
	DB *sql.DB
}

func (repo *UserRepository) CreateUser(id, username, email, hashedPassword string, iban string) error {
	log.Println("Kullanıcı oluşturuluyor:", id, username, email, hashedPassword, iban)
	_, err := repo.DB.Exec("INSERT INTO users (id, fullname, email, password_hash, iban) VALUES (?, ?, ?, ?, ?)", id, username, email, hashedPassword, iban)
	return err
}
