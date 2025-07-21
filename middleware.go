package main

import (
	"context"
	"log"
	"net/http"
	"strings"
)

type contextKey string

const (
	contextKeyUID   contextKey = "userUID"
	contextKeyToken contextKey = "firebaseToken"
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

		claims, err := decodeJWTWithoutValidation(jwtToken)
		if err != nil {
			http.Error(w, "Geçersiz JWT token", http.StatusUnauthorized)
			return
		}

		tokenValue, ok := claims["token"].(string)
		if !ok || tokenValue == "" {
			http.Error(w, "Firebase ID Token bulunamadı", http.StatusUnauthorized)
			return
		}

		token, err := FirebaseAuth.VerifyIDToken(context.Background(), tokenValue)
		if err != nil {
			http.Error(w, "Geçersiz Firebase token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUID, claims["uid"])
		ctx = context.WithValue(ctx, contextKeyToken, token)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
