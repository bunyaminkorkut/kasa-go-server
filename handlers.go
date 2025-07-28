package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func RegisterUserHandler(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Yalnızca POST metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		var req UserRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz JSON formatı", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		req.FullName = strings.TrimSpace(req.FullName)
		req.Email = strings.TrimSpace(req.Email)
		req.Password = strings.TrimSpace(req.Password)

		if req.FullName == "" || req.Email == "" || req.Password == "" {
			http.Error(w, "Tüm alanlar zorunludur", http.StatusBadRequest)
			return
		}

		// 1. Firebase'de kullanıcı oluştur
		firebaseUID, err := CreateFirebaseUser(req.Email, req.Password)
		if err != nil {
			log.Println("Firebase kullanıcı oluşturma hatası:", err)
			http.Error(w, "Kullanıcı Firebase'de oluşturulamadı", http.StatusInternalServerError)
			return
		}

		// 2. Şifreyi hashle
		hashedPwd, err := HashPassword(req.Password)
		if err != nil {
			log.Println("Hashleme hatası:", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		// 3. Veritabanına kaydet
		err = repo.CreateUser(firebaseUID, req.FullName, req.Email, hashedPwd, req.Iban)
		if err != nil {
			log.Println("DB hatası:", err)

			// Firebase'den kullanıcıyı sil
			delErr := DeleteFirebaseUser(firebaseUID)
			if delErr != nil {
				log.Printf("❗ Firebase kullanıcı silinemedi: %v", delErr)
			}

			http.Error(w, "Kullanıcı oluşturulamadı", http.StatusInternalServerError)
			return
		}

		// 4. Firebase Auth token al
		authResult, err := AuthenticateFirebaseUser(req.Email, req.Password)
		if err != nil {
			log.Println("Firebase kimlik doğrulama hatası:", err)
			http.Error(w, "Kimlik doğrulama başarısız", http.StatusUnauthorized)
			return
		}

		// 5. JWT oluştur
		jwtToken, err := generateJWT(map[string]string{
			"uid":       authResult.UID,
			"email":     authResult.Email,
			"token":     authResult.IDToken,
			"expiresIn": authResult.ExpiresIn,
		})
		if err != nil {
			log.Println("JWT oluşturma hatası:", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		// ✅ Başarılı yanıt
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":   "Kullanıcı başarıyla oluşturuldu",
			"jwtToken":  jwtToken,
			"expiresIn": authResult.ExpiresIn + "s",
		})
	}
}

func LoginUserHandler(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Yalnızca POST metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		var req UserRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz JSON formatı", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		req.Email = strings.TrimSpace(req.Email)
		req.Password = strings.TrimSpace(req.Password)

		if req.Email == "" || req.Password == "" {
			http.Error(w, "Email ve şifre zorunludur", http.StatusBadRequest)
			return
		}

		authResult, err := AuthenticateFirebaseUser(req.Email, req.Password)
		if err != nil {
			log.Println("Firebase kimlik doğrulama hatası:", err)
			http.Error(w, "Geçersiz email veya şifre", http.StatusUnauthorized)
			return
		}

		// JWT token oluştur
		jwtToken, err := generateJWT(map[string]string{
			"uid":       authResult.UID,
			"email":     authResult.Email,
			"token":     authResult.IDToken,
			"expiresIn": authResult.ExpiresIn,
		})
		if err != nil {
			log.Println("JWT oluşturma hatası:", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		// Başarılı giriş yanıtı
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":   "Giriş başarılı",
			"jwtToken":  jwtToken,
			"expiresIn": authResult.ExpiresIn + "s",
		})
	}
}

type CreateGroupRequest struct {
	GroupName string `json:"group_name"`
}

func CreateGroupHandler(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Yalnızca POST isteği kabul edilir
		if r.Method != http.MethodPost {
			http.Error(w, "Yalnızca POST metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		// Kimlik doğrulama
		userUID, ok := r.Context().Value("userUID").(string)
		if !ok || userUID == "" {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		// İstek gövdesini çözümle
		var req CreateGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz JSON", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Boş grup adı kontrolü
		req.GroupName = strings.TrimSpace(req.GroupName)
		if req.GroupName == "" {
			http.Error(w, "group_name alanı zorunludur", http.StatusBadRequest)
			return
		}

		// Grup oluştur
		_, err := repo.CreateGroup(userUID, req.GroupName)
		if err != nil {
			log.Println("Grup oluşturulamadı:", err)
			http.Error(w, "Grup oluşturulamadı", http.StatusInternalServerError)
			return
		}

		// Kullanıcının grup listesini getir
		rows, err := repo.getMyGroups(userUID)
		if err != nil {
			log.Println("Grup bilgileri alınamadı:", err)
			http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var groups []map[string]interface{}
		for rows.Next() {
			var groupID int64
			var groupName, creatorID, creatorName, creatorEmail string
			var createdAt int64
			var membersJSON, requestsJSON, expensesJSON, debtsJSON, creditsJSON []byte

			if err := rows.Scan(
				&groupID,
				&groupName,
				&createdAt,
				&creatorID,
				&creatorName,
				&creatorEmail,
				&membersJSON,
				&requestsJSON,
				&expensesJSON,
				&debtsJSON,
				&creditsJSON,
			); err != nil {
				log.Println("Satır okunamadı:", err)
				http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
				return
			}

			var members, requests, expenses, debts, credits []map[string]interface{}
			_ = json.Unmarshal(membersJSON, &members)
			_ = json.Unmarshal(requestsJSON, &requests)
			_ = json.Unmarshal(expensesJSON, &expenses)
			_ = json.Unmarshal(debtsJSON, &debts)
			_ = json.Unmarshal(creditsJSON, &credits)

			groups = append(groups, map[string]interface{}{
				"id":         groupID,
				"name":       groupName,
				"created_at": createdAt,
				"is_admin":   creatorID == userUID,
				"creator": map[string]interface{}{
					"id":       creatorID,
					"fullname": creatorName,
					"email":    creatorEmail,
				},
				"members":          members,
				"pending_requests": requests,
				"expenses":         expenses,
				"debts":            debts,
				"credits":          credits,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)

		// Yanıtı gönder
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(groups); err != nil {
			log.Println("Yanıt gönderilemedi:", err)
		}
	}
}

func GetGroups(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Yalnızca GET metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		userUID := r.Context().Value("userUID")
		if userUID == nil {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		rows, err := repo.getMyGroups(userUID.(string))
		if err != nil {
			log.Println("Grup bilgileri alınamadı:", err)
			http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var groups []map[string]interface{}
		for rows.Next() {
			var groupID int64
			var groupName, creatorID, creatorName, creatorEmail string
			var createdAt int64
			var membersJSON, requestsJSON, expensesJSON, debtsJSON, creditsJSON []byte

			if err := rows.Scan(
				&groupID,
				&groupName,
				&createdAt,
				&creatorID,
				&creatorName,
				&creatorEmail,
				&membersJSON,
				&requestsJSON,
				&expensesJSON,
				&debtsJSON,
				&creditsJSON,
			); err != nil {
				log.Println("Satır okunamadı:", err)
				http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
				return
			}

			var members, requests, expenses, debts, credits []map[string]interface{}
			_ = json.Unmarshal(membersJSON, &members)
			_ = json.Unmarshal(requestsJSON, &requests)
			_ = json.Unmarshal(expensesJSON, &expenses)
			_ = json.Unmarshal(debtsJSON, &debts)
			_ = json.Unmarshal(creditsJSON, &credits)

			groups = append(groups, map[string]interface{}{
				"id":         groupID,
				"name":       groupName,
				"created_at": createdAt,
				"is_admin":   creatorID == userUID.(string),
				"creator": map[string]interface{}{
					"id":       creatorID,
					"fullname": creatorName,
					"email":    creatorEmail,
				},
				"members":          members,
				"pending_requests": requests,
				"expenses":         expenses,
				"debts":            debts,
				"credits":          credits,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
	}
}

type AddGroupRequest struct {
	GroupID     string `json:"group_id"`
	AddedMember string `json:"added_member"`
}

func SendAddRequest(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Yalnızca POST metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		userUID := r.Context().Value("userUID")
		if userUID == nil {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		var req AddGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz JSON", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		req.GroupID = strings.TrimSpace(req.GroupID)
		req.AddedMember = strings.TrimSpace(req.AddedMember)
		if req.AddedMember == "" || req.GroupID == "" {
			http.Error(w, "group_id ve added_member alanları zorunludur", http.StatusBadRequest)
			return
		}

		row, err := repo.sendAddGroupRequest(req.GroupID, req.AddedMember, userUID.(string))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var (
			groupID      int
			groupName    string
			createdTS    int64
			creatorID    string
			creatorName  string
			creatorEmail string
			membersJSON,
			requestsJSON,
			expensesJSON,
			debtsJSON,
			creditsJSON sql.NullString
		)

		err = row.Scan(
			&groupID,
			&groupName,
			&createdTS,
			&creatorID,
			&creatorName,
			&creatorEmail,
			&membersJSON,
			&requestsJSON,
			&expensesJSON,
			&debtsJSON,
			&creditsJSON,
		)
		if err != nil {
			log.Printf("Veri okunurken hata: %v\n", err)
			http.Error(w, "Veriler alınamadı", http.StatusInternalServerError)
			return
		}

		resp := map[string]interface{}{
			"id":         groupID,
			"name":       groupName,
			"created_at": createdTS,
			"is_admin":   creatorID == userUID.(string),
			"creator": map[string]interface{}{
				"id":       creatorID,
				"fullname": creatorName,
				"email":    creatorEmail,
			},
		}

		if membersJSON.Valid {
			var members []map[string]interface{}
			_ = json.Unmarshal([]byte(membersJSON.String), &members)
			resp["members"] = members
		}
		if requestsJSON.Valid {
			var requests []map[string]interface{}
			_ = json.Unmarshal([]byte(requestsJSON.String), &requests)
			resp["pending_requests"] = requests
		}
		if expensesJSON.Valid {
			var expenses []map[string]interface{}
			_ = json.Unmarshal([]byte(expensesJSON.String), &expenses)
			resp["expenses"] = expenses
		}
		if debtsJSON.Valid {
			var debts []map[string]interface{}
			_ = json.Unmarshal([]byte(debtsJSON.String), &debts)
			resp["debts"] = debts
		}
		if creditsJSON.Valid {
			var credits []map[string]interface{}
			_ = json.Unmarshal([]byte(creditsJSON.String), &credits)
			resp["credits"] = credits
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleGetAddRequests(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Yalnızca GET metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		userUID := r.Context().Value("userUID")
		if userUID == nil {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		rows, err := repo.getMyAddRequests(userUID.(string))
		if err != nil {
			http.Error(w, "Grup ekleme istekleri alınamadı", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		requests := make([]map[string]interface{}, 0)
		for rows.Next() {
			var requestID int64
			var groupID int64
			var groupName string
			var requestedAt int64
			var requestStatus string

			if err := rows.Scan(&requestID, &groupID, &groupName, &requestedAt, &requestStatus); err != nil {
				http.Error(w, "Grup ekleme istekleri okunamadı", http.StatusInternalServerError)
				return
			}

			requests = append(requests, map[string]interface{}{
				"request_id":     requestID,
				"group_id":       groupID,
				"group_name":     groupName,
				"requested_at":   requestedAt,
				"request_status": requestStatus,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(requests)
	}
}

type AcceptAddRequest struct {
	RequestID int64 `json:"request_id"`
}

func handleAcceptAddRequest(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Yalnızca POST metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		userUID := r.Context().Value("userUID")
		if userUID == nil {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		var req AcceptAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz JSON", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		log.Printf("📥 request_id geldi: %d\n", req.RequestID)
		err := repo.acceptAddRequest(req.RequestID, userUID.(string))
		if err != nil {
			http.Error(w, "Grup ekleme isteği kabul edilemedi", http.StatusInternalServerError)
			return
		}

		reqRows, err := repo.getMyAddRequests(userUID.(string))
		if err != nil {
			http.Error(w, "Grup ekleme istekleri alınamadı", http.StatusInternalServerError)
			return
		}
		defer reqRows.Close()

		var requests []map[string]interface{}
		for reqRows.Next() {
			var requestID int64
			var groupID int64
			var groupName string
			var requestedAt int64
			var requestStatus string

			if err := reqRows.Scan(&requestID, &groupID, &groupName, &requestedAt, &requestStatus); err != nil {
				http.Error(w, "Grup ekleme istekleri okunamadı", http.StatusInternalServerError)
				return
			}

			requests = append(requests, map[string]interface{}{
				"request_id":     requestID,
				"group_id":       groupID,
				"group_name":     groupName,
				"requested_at":   requestedAt,
				"request_status": requestStatus,
			})
		}

		rows, err := repo.getMyGroups(userUID.(string))
		if err != nil {
			log.Println("Grup bilgileri alınamadı:", err)
			http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var groups []map[string]interface{}
		for rows.Next() {
			var groupID int64
			var groupName, creatorID, creatorName, creatorEmail string
			var createdAt int64
			var membersJSON, requestsJSON, expensesJSON, debtsJSON, creditsJSON []byte

			if err := rows.Scan(
				&groupID,
				&groupName,
				&createdAt,
				&creatorID,
				&creatorName,
				&creatorEmail,
				&membersJSON,
				&requestsJSON,
				&expensesJSON,
				&debtsJSON,
				&creditsJSON,
			); err != nil {
				log.Println("Satır okunamadı:", err)
				http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
				return
			}

			var members, requests, expenses, debts, credits []map[string]interface{}
			_ = json.Unmarshal(membersJSON, &members)
			_ = json.Unmarshal(requestsJSON, &requests)
			_ = json.Unmarshal(expensesJSON, &expenses)
			_ = json.Unmarshal(debtsJSON, &debts)
			_ = json.Unmarshal(creditsJSON, &credits)

			groups = append(groups, map[string]interface{}{
				"id":         groupID,
				"name":       groupName,
				"created_at": createdAt,
				"is_admin":   creatorID == userUID.(string),
				"creator": map[string]interface{}{
					"id":       creatorID,
					"fullname": creatorName,
					"email":    creatorEmail,
				},
				"members":          members,
				"pending_requests": requests,
				"expenses":         expenses,
				"debts":            debts,
				"credits":          credits,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  "Grup ekleme isteği kabul edildi",
			"requests": requests,
			"groups":   groups,
		})
	}
}

type RejectAddRequest struct {
	RequestID int64 `json:"request_id"`
}

func handleRejectAddRequest(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Yalnızca POST metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		userUID := r.Context().Value("userUID")
		if userUID == nil {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		var req RejectAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz JSON", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		err := repo.rejectAddRequest(req.RequestID, userUID.(string))
		if err != nil {
			http.Error(w, "Grup ekleme isteği red edilemedi", http.StatusInternalServerError)
			return
		}

		rows, err := repo.getMyAddRequests(userUID.(string))
		if err != nil {
			http.Error(w, "Grup ekleme istekleri alınamadı", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var requests []map[string]interface{}
		for rows.Next() {
			var requestID int64
			var groupID int64
			var groupName string
			var requestedAt int64
			var requestStatus string

			if err := rows.Scan(&requestID, &groupID, &groupName, &requestedAt, &requestStatus); err != nil {
				http.Error(w, "Grup ekleme istekleri okunamadı", http.StatusInternalServerError)
				return
			}

			requests = append(requests, map[string]interface{}{
				"request_id":     requestID,
				"group_id":       groupID,
				"group_name":     groupName,
				"requested_at":   requestedAt,
				"request_status": requestStatus,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(requests)
	}
}

type ExpenseUser struct {
	UserID string   `json:"user_id"`
	Amount *float64 `json:"amount"`
}

type CreateExpenseRequest struct {
	GroupID      int           `json:"group_id"`
	TotalAmount  float64       `json:"total_amount"`
	Note         string        `json:"note"`
	PaymentTitle string        `json:"payment_title"`
	Users        []ExpenseUser `json:"users"`
	BillImageURL string        `json:"bill_image_url"` // optional
}

func handleCreateGroupExpense(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Sadece POST desteklenir", http.StatusMethodNotAllowed)
			return
		}

		userUIDVal := r.Context().Value("userUID")
		userUID, ok := userUIDVal.(string)
		if !ok || userUID == "" {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		var req CreateExpenseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if req.TotalAmount <= 0 {
			http.Error(w, "Tutar 0'dan büyük olmalı", http.StatusBadRequest)
			return
		}
		if len(req.Users) == 0 {
			http.Error(w, "En az bir katılımcı olmalı", http.StatusBadRequest)
			return
		}

		expense, err := repo.createGroupExpense(r.Context(), userUID, req)
		if err != nil {
			http.Error(w, "Harcama oluşturulamadı: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(expense)
	}
}

func LoginWGoogleHandler(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type RequestBody struct {
			UserID   string `json:"userId"`
			Email    string `json:"email"`
			IDToken  string `json:"idToken"`
			FullName string `json:"fullName"` // opsiyonel ama ilk kayıtta lazım olabilir
			IBAN     string `json:"iban"`     // opsiyonel
		}

		var req RequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz istek", http.StatusBadRequest)
			return
		}

		// === 1. Token doğrulama ve UID/email eşleşmesi ===
		err := ValidateFirebaseTokenWithUser(req.IDToken, req.UserID, req.Email)
		if err != nil {
			http.Error(w, fmt.Sprintf("Firebase doğrulama hatası: %v", err), http.StatusUnauthorized)
			return
		}

		// === 2. Kullanıcı veritabanında var mı kontrol et ===
		user, err := repo.GetUserByID(req.UserID)
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "Veritabanı hatası", http.StatusInternalServerError)
			return
		}

		// === 3. Kullanıcı veritabanında yoksa ekle ===
		if user == nil {
			err := repo.InsertUser(User{
				ID:       req.UserID,
				Email:    req.Email,
				FullName: req.FullName,
				IBAN:     req.IBAN,
			})
			if err != nil {
				http.Error(w, "Kullanıcı kaydı başarısız", http.StatusInternalServerError)
				return
			}
		} else {
			// === 4. Email uyuşmazsa hata ver ===
			if user.Email != req.Email {
				http.Error(w, "Email uyuşmazlığı", http.StatusUnauthorized)
				return
			}
		}

		// === 5. JWT oluştur ===
		token, err := generateJWT(map[string]string{
			"uid":   req.UserID,
			"email": req.Email,
		})
		if err != nil {
			http.Error(w, "JWT oluşturulamadı", http.StatusInternalServerError)
			return
		}

		// === 6. Başarılı yanıt ===
		json.NewEncoder(w).Encode(map[string]string{
			"token": token,
		})
	}
}

func getMeHandler(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Yalnızca GET metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		// JWT'den gelen kullanıcı UID'si
		userUID := r.Context().Value("userUID")
		if userUID == nil {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		// Kullanıcıyı veritabanından al
		user, err := repo.GetUserByID(userUID.(string))
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Kullanıcı bulunamadı", http.StatusNotFound)
				return
			}
			http.Error(w, "Veritabanı hatası", http.StatusInternalServerError)
			return
		}

		// Başarılı yanıt
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

func updateUserHandler(repo *KasaRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "Yalnızca PATCH metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		// Context'ten userUID'yi al ve string olarak atama yap
		userUIDVal := r.Context().Value("userUID")
		userUID, ok := userUIDVal.(string)
		if !ok || userUID == "" {
			http.Error(w, "Yetkisiz erişim: Kullanıcı ID alınamadı", http.StatusUnauthorized)
			return
		}

		var updateData struct {
			FullName *string `json:"fullName,omitempty"`
			IBAN     *string `json:"iban,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			http.Error(w, "Geçersiz JSON formatı", http.StatusBadRequest)
			return
		}

		if updateData.FullName == nil && updateData.IBAN == nil {
			http.Error(w, "Güncellenecek alan belirtilmedi", http.StatusBadRequest)
			return
		}

		user, err := repo.GetUserByID(userUID)
		if err != nil {
			http.Error(w, "Kullanıcı bulunamadı", http.StatusNotFound)
			return
		}

		if updateData.FullName != nil {
			user.FullName = *updateData.FullName
		}
		if updateData.IBAN != nil {
			user.IBAN = *updateData.IBAN
		}

		if err := repo.UpdateUser(user); err != nil {
			http.Error(w, "Kullanıcı güncelleme işlemi başarısız oldu", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}
