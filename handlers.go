package main

import (
	"database/sql"
	"encoding/json"
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
		if r.Method != http.MethodPost {
			http.Error(w, "Yalnızca POST metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		userUID := r.Context().Value("userUID")
		if userUID == nil {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		var req CreateGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz JSON", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		req.GroupName = strings.TrimSpace(req.GroupName)
		if req.GroupName == "" {
			http.Error(w, "group_name alanı zorunludur", http.StatusBadRequest)
			return
		}

		groupID, err := repo.CreateGroup(userUID.(string), req.GroupName)
		if err != nil {
			http.Error(w, "Grup oluşturulamadı", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":    "Grup başarıyla oluşturuldu",
			"group_id":   groupID,
			"group_name": req.GroupName,
		})
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
			var membersJSON, requestsJSON, expensesJSON []byte

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
			); err != nil {
				log.Println("Satır okunamadı:", err)
				http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
				return
			}

			var members, requests, expenses []map[string]interface{}
			_ = json.Unmarshal(membersJSON, &members)
			_ = json.Unmarshal(requestsJSON, &requests)
			_ = json.Unmarshal(expensesJSON, &expenses)

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
			membersJSON  sql.NullString
			requestsJSON sql.NullString
			expensesJSON sql.NullString
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
			if err := json.Unmarshal([]byte(membersJSON.String), &members); err == nil {
				resp["members"] = members
			}
		}

		if requestsJSON.Valid {
			var pending []map[string]interface{}
			if err := json.Unmarshal([]byte(requestsJSON.String), &pending); err == nil {
				resp["pending_requests"] = pending
			}
		}

		if expensesJSON.Valid {
			var expenses []map[string]interface{}
			if err := json.Unmarshal([]byte(expensesJSON.String), &expenses); err == nil {
				resp["expenses"] = expenses
			}
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

		groupRows, err := repo.getMyGroups(userUID.(string))
		if err != nil {
			http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
			return
		}
		defer groupRows.Close()

		var groups []map[string]interface{}
		for groupRows.Next() {
			var groupID int64
			var groupName, creatorID, creatorName, creatorEmail string
			var createdAt int64
			var membersJSON, requestsJSON []byte

			if err := groupRows.Scan(&groupID, &groupName, &createdAt, &creatorID, &creatorName, &creatorEmail, &membersJSON, &requestsJSON); err != nil {
				log.Println("Satır okunamadı:", err)
				http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
				return
			}

			var members, requests []map[string]interface{}
			_ = json.Unmarshal(membersJSON, &members)
			_ = json.Unmarshal(requestsJSON, &requests)

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
		if userUIDVal == nil {
			http.Error(w, "Yetkisiz erişim: userUID bulunamadı", http.StatusUnauthorized)
			return
		}
		userUID, ok := userUIDVal.(string)
		if !ok || userUID == "" {
			http.Error(w, "Yetkisiz erişim: Geçersiz userUID tipi", http.StatusUnauthorized)
			return
		}

		var req CreateExpenseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz JSON formatı: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validasyonları iyileştir
		if req.TotalAmount <= 0 {
			http.Error(w, "Tutar 0'dan büyük olmalıdır", http.StatusBadRequest)
			return
		}
		if len(req.Users) == 0 {
			http.Error(w, "En az bir katılımcı seçilmelidir", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.PaymentTitle) == "" {
			http.Error(w, "Harcama başlığı boş olamaz", http.StatusBadRequest)
			return
		}

		// Group expense'i oluştur ve güncel grup verisini al
		row, err := repo.createGroupExpenseAndReturnGroupRow(r.Context(), userUID, req)
		if err != nil {
			// Özel hata mesajları
			if strings.Contains(err.Error(), "katılımcı tutarları toplamı") {
				http.Error(w, "Katılımcı tutarları toplamı genel tutar ile eşleşmiyor", http.StatusBadRequest)
				return
			}
			// Catch our specific error for nil amounts
			if strings.Contains(err.Error(), "participant amount cannot be null") {
				http.Error(w, "Katılımcı tutarı boş olamaz", http.StatusBadRequest)
				return
			}
			http.Error(w, "Harcama oluşturulamadı: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Scan işlemi için geçici değişkenler
		var (
			groupID, groupName, creatorID, creatorName, creatorEmail string
			createdTs                                                int64
			membersJSON, pendingRequestsJSON, expensesJSON           sql.NullString
		)

		// Row'dan verileri scan et
		if err := row.Scan(
			&groupID,
			&groupName,
			&createdTs,
			&creatorID,
			&creatorName,
			&creatorEmail,
			&membersJSON,
			&pendingRequestsJSON,
			&expensesJSON,
		); err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Grup bulunamadı (veritabanı tutarsızlığı olabilir)", http.StatusNotFound)
				return
			}
			http.Error(w, "Grup verisi işlenemedi: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// JSON verilerini parse et
		var (
			members         json.RawMessage // Use json.RawMessage to keep original JSON structure or decode into specific structs
			pendingRequests json.RawMessage
			expenses        json.RawMessage
		)

		// Members JSON'ını parse et
		// Check .Valid first to avoid dereferencing NullString's String field if it's not valid
		if membersJSON.Valid && membersJSON.String != "null" {
			members = json.RawMessage(membersJSON.String)
		} else {
			members = json.RawMessage("[]") // Default to empty array if null or invalid
		}

		// Pending requests JSON'ını parse et
		if pendingRequestsJSON.Valid && pendingRequestsJSON.String != "null" {
			pendingRequests = json.RawMessage(pendingRequestsJSON.String)
		} else {
			pendingRequests = json.RawMessage("[]")
		}

		// Expenses JSON'ını parse et
		if expensesJSON.Valid && expensesJSON.String != "null" {
			expenses = json.RawMessage(expensesJSON.String)
		} else {
			expenses = json.RawMessage("[]")
		}

		// Response data'sını oluştur
		groupData := map[string]interface{}{
			"group_id":         groupID,
			"group_name":       groupName,
			"created_ts":       createdTs,
			"creator_id":       creatorID,
			"creator_name":     creatorName,
			"creator_email":    creatorEmail,
			"members":          members,         // json.RawMessage will be directly embedded
			"pending_requests": pendingRequests, // json.RawMessage will be directly embedded
			"expenses":         expenses,        // json.RawMessage will be directly embedded
		}

		// Başarılı response döndür
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(groupData); err != nil {
			// Bu noktada header zaten gönderildi, log'a yazmak en iyisi
			// log.Printf("Error encoding response: %v", err) // Use your logging framework
			return
		}
	}
}
