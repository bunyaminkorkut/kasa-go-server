package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func RegisterUserHandler(repo *UserRepository) http.HandlerFunc {
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

func LoginUserHandler(repo *UserRepository) http.HandlerFunc {
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
