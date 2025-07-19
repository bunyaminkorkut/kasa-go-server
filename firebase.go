package main

import (
	"context"
	"fmt"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

func connectToFirebase() {
	ctx := context.Background()

	credsFile := os.Getenv("FIREBASE_CREDENTIALS")
	if credsFile == "" {
		log.Fatal("❌ FIREBASE_CREDENTIALS .env içinde tanımlı değil")
	}

	opt := option.WithCredentialsFile(credsFile)

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("❌ Firebase başlatılamadı: %v", err)
	}

	_, err = app.Firestore(ctx)
	if err != nil {
		log.Fatalf("❌ Firestore bağlantısı başarısız: %v", err)
	}

	fmt.Println("✅ Firebase'e başarıyla bağlanıldı")
}
func CreateFirebaseUser(email, password string) (string, error) {
	ctx := context.Background()

	credsFile := os.Getenv("FIREBASE_CREDENTIALS")
	if credsFile == "" {
		return "", fmt.Errorf("❌ FIREBASE_CREDENTIALS .env içinde tanımlı değil")
	}

	opt := option.WithCredentialsFile(credsFile)

	// Firebase app yalnızca bir kere oluşturulmalı (singleton pattern önerilir)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return "", fmt.Errorf("❌ Firebase başlatılamadı: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return "", fmt.Errorf("❌ Firebase Auth başlatılamadı: %w", err)
	}

	params := (&auth.UserToCreate{}).
		Email(email).
		Password(password)

	userRecord, err := authClient.CreateUser(ctx, params)
	if err != nil {
		return "", fmt.Errorf("❌ Firebase kullanıcı oluşturulamadı: %w", err)
	}

	return userRecord.UID, nil
}
