package main

import (
	"context"
	"net/http"
	"strings"
)

// decodeJWTWithoutValidation fonksiyonunu kendi içinde implemente etmelisin
// Örneği aşağıda açıklayabilirim istersen.

func AuthMiddleware(next http.Handler, repo *KasaRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Yetkisiz erişim: token eksik", http.StatusUnauthorized)
			return
		}

		jwtToken := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := decodeJWTWithoutValidation(jwtToken)
		if err != nil {
			http.Error(w, "Geçersiz JWT token", http.StatusUnauthorized)
			return
		}

		uid, ok := claims["uid"].(string)
		if !ok || uid == "" {
			http.Error(w, "Geçersiz JWT token: UID eksik", http.StatusUnauthorized)
			return
		}

		email, _ := claims["email"].(string)

		rows, err := repo.GetUserByEmail(email)
		if err != nil {
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var userID string
		if rows.Next() {
			if err := rows.Scan(&userID); err != nil {
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			if userID != uid {
				http.Error(w, "Yetkisiz erişim: UID uyuşmuyor", http.StatusUnauthorized)
				return
			}
		} else {
			http.Error(w, "Kullanıcı bulunamadı", http.StatusNotFound)
			return
		}

		// Context'e UID ve email ekle
		ctx := context.WithValue(r.Context(), "userUID", uid)
		ctx = context.WithValue(ctx, "email", email)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
