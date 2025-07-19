package main

import (
	"context"
	"fmt"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
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
