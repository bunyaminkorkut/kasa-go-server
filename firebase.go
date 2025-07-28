package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

var FirebaseAuth *auth.Client

// FirebaseConfig yapısı FIREBASE_CONFIG JSON'undaki veriyi temsil eder
type FirebaseConfig struct {
	APIKey string `json:"apiKey"`
}

func connectToFirebase(ctx context.Context) (*auth.Client, error) {
	credsFile := os.Getenv("FIREBASE_CREDENTIALS")
	if credsFile == "" {
		return nil, fmt.Errorf("❌ FIREBASE_CREDENTIALS .env içinde tanımlı değil")
	}

	opt := option.WithCredentialsFile(credsFile)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("❌ Firebase başlatılamadı: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("❌ Firebase Auth başlatılamadı: %w", err)
	}

	return authClient, nil
}

func CreateFirebaseUser(email, password string) (string, error) {
	ctx := context.Background()

	credsFile := os.Getenv("FIREBASE_CREDENTIALS")
	if credsFile == "" {
		return "", fmt.Errorf("❌ FIREBASE_CREDENTIALS .env içinde tanımlı değil")
	}

	opt := option.WithCredentialsFile(credsFile)

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

func DeleteFirebaseUser(uid string) error {
	ctx := context.Background()

	credsFile := os.Getenv("FIREBASE_CREDENTIALS")
	if credsFile == "" {
		return fmt.Errorf("❌ FIREBASE_CREDENTIALS .env içinde tanımlı değil")
	}

	opt := option.WithCredentialsFile(credsFile)

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return fmt.Errorf("❌ Firebase başlatılamadı: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return fmt.Errorf("❌ Firebase Auth başlatılamadı: %w", err)
	}

	return authClient.DeleteUser(ctx, uid)
}

func AuthenticateFirebaseUser(email, password string) (FirebaseAuthResult, error) {
	configPath := os.Getenv("FIREBASE_CONFIG")
	if configPath == "" {
		return FirebaseAuthResult{}, fmt.Errorf("❌ FIREBASE_CONFIG_PATH tanımlı değil")
	}

	// Dosyadan config'i oku
	data, err := os.ReadFile(configPath)
	if err != nil {
		return FirebaseAuthResult{}, fmt.Errorf("❌ FIREBASE_CONFIG_PATH dosyası okunamadı: %w", err)
	}

	var config FirebaseConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return FirebaseAuthResult{}, fmt.Errorf("❌ FIREBASE_CONFIG JSON parse edilemedi: %w", err)
	}

	if config.APIKey == "" {
		return FirebaseAuthResult{}, fmt.Errorf("❌ apiKey FIREBASE_CONFIG içinde bulunamadı")
	}

	url := fmt.Sprintf("https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=%s", config.APIKey)

	payload := map[string]interface{}{
		"email":             email,
		"password":          password,
		"returnSecureToken": true,
	}
	payloadBytes, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return FirebaseAuthResult{}, fmt.Errorf("❌ HTTP isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		IDToken   string `json:"idToken"`
		ExpiresIn string `json:"expiresIn"`
		LocalID   string `json:"localId"`
		Email     string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return FirebaseAuthResult{}, fmt.Errorf("❌ Yanıt çözümlenemedi: %w", err)
	}

	if result.IDToken == "" {
		return FirebaseAuthResult{}, fmt.Errorf("❌ Giriş başarısız, ID token alınamadı")
	}
	log.Println("Firebase kimlik doğrulama başarılı:", result)

	return FirebaseAuthResult{
		IDToken:   result.IDToken,
		ExpiresIn: result.ExpiresIn,
		UID:       result.LocalID,
		Email:     result.Email,
	}, nil
}

type FirebaseAuthResult struct {
	IDToken   string
	ExpiresIn string
	UID       string
	Email     string
}

func ValidateFirebaseTokenWithUser(idToken, expectedUID, expectedEmail string) error {
	ctx := context.Background()

	credsFile := os.Getenv("FIREBASE_CREDENTIALS")
	if credsFile == "" {
		return fmt.Errorf("❌ FIREBASE_CREDENTIALS .env içinde tanımlı değil")
	}

	opt := option.WithCredentialsFile(credsFile)

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return fmt.Errorf("❌ Firebase başlatılamadı: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return fmt.Errorf("❌ Firebase Auth başlatılamadı: %w", err)
	}

	token, err := authClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		return fmt.Errorf("❌ Token doğrulama başarısız: %w", err)
	}

	// UID kontrolü
	if token.UID != expectedUID {
		return fmt.Errorf("❌ UID uyuşmuyor. Token UID: %s, Beklenen UID: %s", token.UID, expectedUID)
	}

	// Email kontrolü (token.Claims içinde olabilir)
	emailClaim, ok := token.Claims["email"].(string)
	if !ok || emailClaim != expectedEmail {
		return fmt.Errorf("❌ Email uyuşmuyor. Token Email: %v, Beklenen Email: %s", emailClaim, expectedEmail)
	}

	return nil // hepsi doğru
}
