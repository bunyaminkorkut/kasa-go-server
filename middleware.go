package main

import (
	"context"
	"log"
	"net/http"
	"strings"
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

		idToken := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := FirebaseAuth.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			http.Error(w, "Geçersiz token", http.StatusUnauthorized)
			return
		}

		// UID gibi bilgileri context'e ekle
		ctx := context.WithValue(r.Context(), "userUID", token.UID)
		ctx = context.WithValue(ctx, "firebaseToken", token)

		// Bir sonraki handler'a devam et
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
