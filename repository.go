package main

import (
	"database/sql"
	"errors"
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

func (repo *KasaRepository) getMyGroups(userID string) (*sql.Rows, error) {
	return repo.DB.Query(`
		SELECT 
			g.id AS group_id,
			g.group_name,
			UNIX_TIMESTAMP(g.created_at) AS created_ts,
			u.id AS creator_id,
			u.fullname AS creator_name,
			u.email AS creator_email,

			(
				SELECT JSON_ARRAYAGG(JSON_OBJECT(
					'id', gm_user.id,
					'fullname', gm_user.fullname,
					'email', gm_user.email
				))
				FROM group_members gm
				JOIN users gm_user ON gm.user_id = gm_user.id
				WHERE gm.group_id = g.id
			) AS members,

			(
				SELECT JSON_ARRAYAGG(JSON_OBJECT(
					'request_id', r.request_id,
					'user_id', r.user_id,
					'fullname', ru.fullname,
					'email', ru.email,
					'requested_at', UNIX_TIMESTAMP(r.requested_at),
					'request_status', r.request_status,
					'group_name', gr.group_name,
					'group_id', gr.id
				))
				FROM group_add_requests r
				JOIN users ru ON r.user_id = ru.id
				JOIN groups gr ON r.group_id = gr.id 
				WHERE r.group_id = g.id AND r.request_status = 'pending'
			) AS pending_requests,

			(
				SELECT JSON_ARRAYAGG(
					JSON_OBJECT(
						'expense_id', e.expense_id,
						'amount', e.amount,
						'description_note', e.description_note,
						'payment_date', UNIX_TIMESTAMP(e.payment_date),
						'payment_title', e.payment_title,
						'bill_image_url', e.bill_image_url,
						'payer_id', e.payer_id,
						'participants', (
							SELECT JSON_ARRAYAGG(
								JSON_OBJECT(
									'user_id', p.user_id,
									'amount_share', p.amount_share,
									'payment_status', p.payment_status
								)
							)
							FROM group_expense_participants p
							WHERE p.expense_id = e.expense_id
						)
					)
				)
				FROM group_expenses e
				WHERE e.group_id = g.id
			) AS expenses

		FROM groups g
		JOIN users u ON g.creator_id = u.id
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = ?
		GROUP BY g.id
		ORDER BY g.created_at DESC
	`, userID)
}

func (repo *KasaRepository) sendAddGroupRequest(groupID, addedMemberEmail, currentUserID string) (*sql.Row, error) {
	// Email'e karşılık gelen kullanıcı ID'sini al
	var addedMemberID string
	err := repo.DB.QueryRow("SELECT id FROM users WHERE email = ?", addedMemberEmail).Scan(&addedMemberID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Kullanıcı bulunamadı: %s\n", addedMemberEmail)
			return nil, fmt.Errorf("kullanıcı bulunamadı: %s", addedMemberEmail)
		}
		log.Println("Kullanıcı kontrolü sırasında hata:", err)
		return nil, err
	}

	// Aynı istek zaten varsa tekrar ekleme
	var count int
	err = repo.DB.QueryRow(`
		SELECT COUNT(*) FROM group_add_requests 
		WHERE group_id = ? AND user_id = ? AND request_status = 'pending'
	`, groupID, addedMemberID).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("mevcut istek kontrolü sırasında hata: %v", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("bu kullanıcıya zaten bekleyen bir istek gönderilmiş")
	}

	// Grup ekleme isteğini gönder
	_, err = repo.DB.Exec(
		"INSERT INTO group_add_requests (group_id, user_id) VALUES (?, ?)",
		groupID, addedMemberID,
	)
	if err != nil {
		log.Println("Grup ekleme isteği gönderilemedi:", err)
		return nil, err
	}

	// Güncel grup bilgilerini çek (expenses ve participants dahil)
	row := repo.DB.QueryRow(`
		SELECT 
			g.id AS group_id,
			g.group_name,
			UNIX_TIMESTAMP(g.created_at) AS created_ts,
			u.id AS creator_id,
			u.fullname AS creator_name,
			u.email AS creator_email,

			(
				SELECT JSON_ARRAYAGG(JSON_OBJECT(
					'id', gm_user.id,
					'fullname', gm_user.fullname,
					'email', gm_user.email
				))
				FROM group_members gm
				JOIN users gm_user ON gm.user_id = gm_user.id
				WHERE gm.group_id = g.id
			) AS members,

			(
				SELECT JSON_ARRAYAGG(JSON_OBJECT(
					'request_id', r.request_id,
					'user_id', r.user_id,
					'fullname', ru.fullname,
					'email', ru.email,
					'requested_at', UNIX_TIMESTAMP(r.requested_at),
					'request_status', r.request_status,
					'group_name', gr.group_name,
					'group_id', gr.id
				))
				FROM group_add_requests r
				JOIN users ru ON r.user_id = ru.id
				JOIN groups gr ON r.group_id = gr.id 
				WHERE r.group_id = g.id AND r.request_status = 'pending'
			) AS pending_requests,

			(
				SELECT JSON_ARRAYAGG(JSON_OBJECT(
					'expense_id', e.expense_id,
					'payer_id', e.payer_id,
					'payer_name', p.fullname,
					'amount', e.amount,
					'description_note', e.description_note,
					'payment_title', e.payment_title,
					'payment_date', UNIX_TIMESTAMP(e.payment_date),
					'bill_image_url', e.bill_image_url,
					'participants', (
						SELECT JSON_ARRAYAGG(JSON_OBJECT(
							'user_id', ep.user_id,
							'user_name', up.fullname,
							'amount_share', ep.amount_share,
							'payment_status', ep.payment_status
						))
						FROM group_expense_participants ep
						LEFT JOIN users up ON ep.user_id = up.id
						WHERE ep.expense_id = e.expense_id
					)
				))
				FROM group_expenses e
				LEFT JOIN users p ON e.payer_id = p.id
				WHERE e.group_id = g.id
			) AS expenses

		FROM groups g
		JOIN users u ON g.creator_id = u.id
		WHERE g.id = ?
	`, groupID)
	return row, nil
}

func (repo *KasaRepository) getMyAddRequests(userID string) (*sql.Rows, error) {
	rows, err := repo.DB.Query(`
		SELECT gar.request_id, gar.group_id, g.group_name, UNIX_TIMESTAMP(gar.requested_at), gar.request_status
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

	// 1. Gerekli bilgileri al (sadece 'pending' durumundaki istekler işlenir)
	err = tx.QueryRow(`
		SELECT group_id, user_id 
		FROM group_add_requests 
		WHERE request_id = ? AND request_status = 'pending'
	`, requestID).Scan(&groupID, &reqUserID)

	if err == sql.ErrNoRows {
		tx.Rollback()
		log.Printf("Geçersiz ya da işlenmiş istek: request_id=%d\n", requestID)
		return fmt.Errorf("bu istek zaten işlenmiş veya mevcut değil")
	} else if err != nil {
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
	_, err = tx.Exec(`
		UPDATE group_add_requests 
		SET request_status = 'accepted' 
		WHERE request_id = ?
	`, requestID)
	if err != nil {
		tx.Rollback()
		log.Println("Grup ekleme isteği güncellenemedi:", err)
		return err
	}

	// 4. Kullanıcıyı gruba ekle
	_, err = tx.Exec(`
		INSERT INTO group_members (group_id, user_id) 
		VALUES (?, ?)
	`, groupID, userID)
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

	log.Printf("✅ İstek kabul edildi: request_id=%d, user_id=%s\n", requestID, userID)
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

func (repo *KasaRepository) createGroupExpenseAndReturnGroupRow(payerID string, req CreateExpenseRequest) (*sql.Row, error) {
	tx, err := repo.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	result, err := tx.Exec(
		`INSERT INTO group_expenses (group_id, payer_id, amount, description_note, payment_title, bill_image_url, payment_date)
		 VALUES (?, ?, ?, ?, ?, ?, NOW())`,
		req.GroupID, payerID, req.TotalAmount, req.Note, req.PaymentTitle, req.BillImageURL,
	)
	if err != nil {
		return nil, err
	}

	expenseID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	var shareAmounts []float64
	allHaveAmount := true

	for _, user := range req.Users {
		if user.Amount == nil {
			allHaveAmount = false
			break
		}
		shareAmounts = append(shareAmounts, *user.Amount)
	}

	if allHaveAmount {
		var sum float64
		for _, amount := range shareAmounts {
			sum += amount
		}
		if int(sum*100) != int(req.TotalAmount*100) {
			return nil, errors.New("katılımcı tutarları toplamı total_amount ile eşleşmiyor")
		}
	} else {
		count := float64(len(req.Users))
		share := req.TotalAmount / count
		for i := range req.Users {
			req.Users[i].Amount = &share
		}
	}

	stmt, err := tx.Prepare("INSERT INTO group_expense_participants (expense_id, user_id, amount_share, payment_status) VALUES (?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for _, user := range req.Users {
		paymentStatus := "unpaid"
		if user.UserID == payerID {
			paymentStatus = "paid"
		}

		_, err := stmt.Exec(expenseID, user.UserID, *user.Amount, paymentStatus)
		if err != nil {
			return nil, err
		}
	}

	// Transaction içinde query çalıştır
	row := tx.QueryRow(`
		SELECT 
			g.id AS group_id,
			g.group_name,
			UNIX_TIMESTAMP(g.created_at) AS created_ts,
			u.id AS creator_id,
			u.fullname AS creator_name,
			u.email AS creator_email,

			(
				SELECT JSON_ARRAYAGG(JSON_OBJECT(
					'id', gm_user.id,
					'fullname', gm_user.fullname,
					'email', gm_user.email
				))
				FROM group_members gm
				JOIN users gm_user ON gm.user_id = gm_user.id
				WHERE gm.group_id = g.id
			) AS members,

			(
				SELECT JSON_ARRAYAGG(JSON_OBJECT(
					'request_id', r.request_id,
					'user_id', r.user_id,
					'fullname', ru.fullname,
					'email', ru.email,
					'requested_at', UNIX_TIMESTAMP(r.requested_at),
					'request_status', r.request_status,
					'group_name', gr.group_name,
					'group_id', gr.id
				))
				FROM group_add_requests r
				JOIN users ru ON r.user_id = ru.id
				JOIN groups gr ON r.group_id = gr.id 
				WHERE r.group_id = g.id AND r.request_status = 'pending'
			) AS pending_requests,

			(
				SELECT JSON_ARRAYAGG(JSON_OBJECT(
					'expense_id', e.expense_id,
					'payer_id', e.payer_id,
					'payer_name', p.fullname,
					'amount', e.amount,
					'description_note', e.description_note,
					'payment_title', e.payment_title,
					'payment_date', UNIX_TIMESTAMP(e.payment_date),
					'bill_image_url', e.bill_image_url,
					'participants', (
						SELECT JSON_ARRAYAGG(JSON_OBJECT(
							'user_id', ep.user_id,
							'user_name', up.fullname,
							'amount_share', ep.amount_share,
							'payment_status', ep.payment_status
						))
						FROM group_expense_participants ep
						LEFT JOIN users up ON ep.user_id = up.id
						WHERE ep.expense_id = e.expense_id
					)
				))
				FROM group_expenses e
				LEFT JOIN users p ON e.payer_id = p.id
				WHERE e.group_id = g.id
				ORDER BY e.payment_date DESC
			) AS expenses

		FROM groups g
		JOIN users u ON g.creator_id = u.id
		WHERE g.id = ?
	`, req.GroupID)

	return row, nil
}
