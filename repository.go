package main

import (
	"database/sql"
	"log"
)

type KasaRepository struct {
	DB *sql.DB
}

func (repo *KasaRepository) CreateUser(id, username, email, hashedPassword string, iban string) error {
	log.Println("Kullanıcı oluşturuluyor:", id, username, email, hashedPassword, iban)
	_, err := repo.DB.Exec("INSERT INTO users (id, fullname, email, password_hash, iban) VALUES (?, ?, ?, ?, ?)", id, username, email, hashedPassword, iban)
	return err
}

func (repo *KasaRepository) CreateGroup(creatorID, groupName string) (int64, error) {
	result, err := repo.DB.Exec("INSERT INTO groups (group_name, creator_id) VALUES (?, ?)", groupName, creatorID)
	if err != nil {
		log.Println("Grup oluşturma hatası:", err)
		return 0, err
	}

	groupID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	_, err = repo.DB.Exec("INSERT INTO group_members (group_id, user_id) VALUES (?, ?)", groupID, creatorID)
	if err != nil {
		log.Println("Grup üyesi ekleme hatası:", err)
		return 0, err
	}

	return groupID, nil
}

func (repo *KasaRepository) GetUserByEmail(email string) (*sql.Rows, error) {
	return repo.DB.Query("SELECT id FROM users WHERE email = ?", email)
}

func (repo *KasaRepository) GetMyGroups(userID string) (*sql.Rows, error) {
	rows, err := repo.DB.Query(`
    SELECT g.id, g.group_name, UNIX_TIMESTAMP(g.created_at) as created_ts
    FROM groups g 
    JOIN group_members gm ON g.id = gm.group_id 
    WHERE gm.user_id = ?
	Order by g.created_at desc
`, userID)

	if err != nil {
		log.Println("Grup bilgileri alınamadı:", err)
		return nil, err
	}
	return rows, nil
}
