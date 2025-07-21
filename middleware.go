package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	// jwt kütüphanesi, import etmen gerek
)

func FirebaseAuthMiddleware(next http.Handler) http.Handler {
	if FirebaseAuth == nil {
		ctx := context.Background()
		var err error
		FirebaseAuth, err = connectToFirebase(ctx)
		if err != nil {
			log.Fatalf("❌ Firebase Auth başlatılamadı: %v", err)
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Yetkisiz erişim: token eksik", http.StatusUnauthorized)
			return
		}

		jwtToken := strings.TrimPrefix(authHeader, "Bearer ")

		// Burada JWT'yi doğrulama değil sadece decode yapıyoruz (doğrulama isteğe bağlı)
		claims, err := decodeJWTWithoutValidation(jwtToken)
		if err != nil {
			http.Error(w, "Geçersiz JWT token", http.StatusUnauthorized)
			return
		}

		// claims["token"] alanını alıyoruz, burada gerçek Firebase ID Token olmalı
		tokenValue, ok := claims["token"].(string)
		if !ok || tokenValue == "" {
			http.Error(w, "Firebase ID Token bulunamadı", http.StatusUnauthorized)
			return
		}

		// Firebase ID Token'ı doğrula
		token, err := FirebaseAuth.VerifyIDToken(context.Background(), tokenValue)
		if err != nil {
			http.Error(w, "Geçersiz Firebase token", http.StatusUnauthorized)
			return
		}

		// Context'e kullanıcı UID'sini ve token bilgisini ekle
		ctx := context.WithValue(r.Context(), "userUID", claims["uid"])
		ctx = context.WithValue(ctx, "firebaseToken", token)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
