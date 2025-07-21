package main

import (
	"database/sql"
	"fmt"
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

func (repo *KasaRepository) sendAddGroupRequest(groupID, addedMemberEmail string) error {
	// Email'e karşılık gelen kullanıcı ID'sini al
	var addedMemberID string
	err := repo.DB.QueryRow("SELECT id FROM users WHERE email = ?", addedMemberEmail).Scan(&addedMemberID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Kullanıcı bulunamadı: %s\n", addedMemberEmail)
			return fmt.Errorf("kullanıcı bulunamadı: %s", addedMemberEmail)
		}
		log.Println("Kullanıcı kontrolü sırasında hata:", err)
		return err
	}

	// Grup ekleme isteğini gönder
	_, err = repo.DB.Exec(
		"INSERT INTO group_add_requests (group_id, user_id) VALUES (?, ?)",
		groupID, addedMemberID,
	)
	if err != nil {
		log.Println("Grup ekleme isteği gönderilemedi:", err)
		return err
	}

	return nil
}

func (repo *KasaRepository) getMyAddRequests(userID string) (*sql.Rows, error) {
	rows, err := repo.DB.Query(`
	SELECT *
	FROM group_add_requests gar
	JOIN groups g ON gar.group_id = g.id
	WHERE gar.user_id = ?
	ORDER BY gar.requested_at DESC
`, userID)

	if err != nil {
		log.Println("Grup ekleme istekleri alınamadı:", err)
		return nil, err
	}
	return rows, nil
}

func (repo *KasaRepository) acceptAddRequest(requestID int64, userID string) error {
	tx, err := repo.DB.Begin()
	if err != nil {
		log.Println("Transaction başlatılamadı:", err)
		return err
	}

	var groupID int64
	var reqUserID string

	// 1. Gerekli bilgileri al (group_id ve user_id)
	err = tx.QueryRow("SELECT group_id, user_id FROM group_add_requests WHERE request_id = ?", requestID).Scan(&groupID, &reqUserID)
	if err != nil {
		tx.Rollback()
		log.Println("Grup ID veya kullanıcı ID alınamadı:", err)
		return err
	}

	// 2. userID doğruluğunu kontrol et
	if reqUserID != userID {
		tx.Rollback()
		log.Printf("Yetkisiz işlem: parametre userID '%s' != veritabanı userID '%s'\n", userID, reqUserID)
		return fmt.Errorf("yetkisiz işlem: kullanıcı uyuşmazlığı")
	}

	// 3. İsteği 'accepted' olarak güncelle
	_, err = tx.Exec("UPDATE group_add_requests SET request_status = 'accepted' WHERE request_id = ?", requestID)
	if err != nil {
		tx.Rollback()
		log.Println("Grup ekleme isteği güncellenemedi:", err)
		return err
	}

	// 4. Kullanıcıyı gruba ekle
	_, err = tx.Exec("INSERT INTO group_members (group_id, user_id) VALUES (?, ?)", groupID, userID)
	if err != nil {
		tx.Rollback()
		log.Println("Kullanıcı gruba eklenemedi:", err)
		return err
	}

	// 5. Commit işlemi
	err = tx.Commit()
	if err != nil {
		log.Println("Transaction commit edilemedi:", err)
		return err
	}

	return nil
}

func (repo *KasaRepository) rejectAddRequest(requestID int64, userID string) error {
	tx, err := repo.DB.Begin()
	if err != nil {
		log.Println("Transaction başlatılamadı:", err)
		return err
	}

	var reqUserID string

	// 1. İstek sahibi kim kontrol et
	err = tx.QueryRow("SELECT user_id FROM group_add_requests WHERE request_id = ?", requestID).Scan(&reqUserID)
	if err != nil {
		tx.Rollback()
		log.Println("İstek bilgisi alınamadı:", err)
		return err
	}

	// 2. userID doğruluğunu kontrol et
	if reqUserID != userID {
		tx.Rollback()
		log.Printf("Yetkisiz işlem: parametre userID '%s' != veritabanı userID '%s'\n", userID, reqUserID)
		return fmt.Errorf("yetkisiz işlem: kullanıcı uyuşmazlığı")
	}

	// 3. İsteği 'rejected' olarak güncelle
	_, err = tx.Exec("UPDATE group_add_requests SET request_status = 'rejected' WHERE request_id = ?", requestID)
	if err != nil {
		tx.Rollback()
		log.Println("Grup ekleme isteği reddedilemedi:", err)
		return err
	}

	// 4. Commit işlemi
	err = tx.Commit()
	if err != nil {
		log.Println("Transaction commit edilemedi:", err)
		return err
	}

	return nil
}
