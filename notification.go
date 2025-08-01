package main

import (
	"context"
	"fmt"
	"log"

	"firebase.google.com/go/v4/messaging"
)

func SendNotification(ctx context.Context, repo *KasaRepository, userID, title, body string, data map[string]string) error {
	if FirebaseMessagingClient == nil {
		return fmt.Errorf("FirebaseMessagingClient initialize edilmemiş")
	}

	// Token'ı çek
	userToken, err := repo.GetFCMTokenByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("FCM token alınamadı: %w", err)
	}

	if userToken == "" {
		log.Printf("Kullanıcı (%s) için FCM token bulunamadı, bildirim gönderilmiyor.", userID)
		return nil
	}

	badge := 1

	message := &messaging.Message{
		Token: userToken,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data, // 🔥 Bildirimle birlikte yönlendirme verileri buraya
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: title,
						Body:  body,
					},
					Badge: &badge,
					Sound: "default",
				},
			},
		},
	}

	resp, err := FirebaseMessagingClient.Send(ctx, message)
	if err != nil {
		log.Printf("Bildirim gönderilemedi: %v", err)
		return err
	}

	log.Printf("Bildirim gönderildi, mesaj ID: %s", resp)
	return nil
}
