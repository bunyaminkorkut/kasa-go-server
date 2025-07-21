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

func (repo *UserRepository) CreateGroup(creatorID, groupName string) (int64, error) {
	result, err := repo.DB.Exec("INSERT INTO groups (group_name, creator_id) VALUES (?, ?)", groupName, creatorID)
	if err != nil {
		return 0, err
	}

	groupID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, err = repo.DB.Exec("INSERT INTO group_members (group_id, user_id) VALUES (?, ?)", groupID, creatorID)
	if err != nil {
		return 0, err
	}

	return groupID, nil
}
