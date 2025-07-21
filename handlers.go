package main

import (
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
			http.Error(w, "Yalnızca POST metodu desteklenir", http.StatusMethodNotAllowed)
			return
		}

		userUID := r.Context().Value("userUID")
		if userUID == nil {
			http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
			return
		}

		rows, err := repo.GetMyGroups(userUID.(string))

		if err != nil {
			http.Error(w, "Grup bilgileri alınamadı", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var groups []map[string]interface{}
		for rows.Next() {
			var groupID int64
			var groupName string
			var createdAt int64
			if err := rows.Scan(&groupID, &groupName, &createdAt); err != nil {
				http.Error(w, "Grup bilgileri okunamadı", http.StatusInternalServerError)
				return
			}
			groups = append(groups, map[string]interface{}{
				"id":   groupID,
				"name": groupName,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
	}
}
