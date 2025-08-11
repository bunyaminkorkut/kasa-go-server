package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/messaging"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

var FirebaseMessagingClient *messaging.Client
var FirebaseAuth *auth.Client

func main() {
	// .env dosyasını yükle
	if err := godotenv.Load(); err != nil {
		log.Fatal("❌ .env dosyası yüklenemedi:", err)
	}

	// Ortam değişkenlerini oku
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASS")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	dbname := os.Getenv("DB_NAME")

	// DSN oluştur
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, host, port, dbname)

	// Veritabanına bağlan
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("❌ Veritabanına bağlanılamadı:", err)
	}
	defer db.Close()

	// Bağlantı testi
	if err := db.Ping(); err != nil {
		log.Fatal("❌ Veritabanı bağlantısı başarısız:", err)
	}

	// SQL scriptini oku
	schema, err := os.ReadFile("./database/schema.sql")
	if err != nil {
		log.Fatal("❌ schema.sql dosyası okunamadı:", err)
	}

	// SQL'i çalıştır (tek tek komutları ayırarak güvenli hale getiriyoruz)
	queries := string(schema)
	for _, stmt := range splitSQL(queries) {
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("❌ SQL çalıştırma hatası:\n%v\nHata: %v", stmt, err)
		}
	}
	fmt.Println("✅ SQL script başarıyla çalıştırıldı.")
	clients, err := connectToFirebase(context.Background())
	if err != nil {
		log.Fatal("❌ Firebase bağlantısı başarısız:", err)
	}
	FirebaseAuth = clients.AuthClient
	FirebaseMessagingClient = clients.MessagingClient

	// Sunucu başlatma
	repo := &KasaRepository{DB: db}

	// HTTP endpointleri
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, `
        <!DOCTYPE html>
        <html>
        <head>
            <title>Deneme Sayfası</title>
        </head>
        <body>
            <h1>Hoşgeldin!</h1>
            <p>Bu bir basit HTML sayfasıdır.</p>
        </body>
        </html>
    `)
	})

	http.HandleFunc("/v", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Merhaba, test serverı çalışıyor!")
	})
	http.HandleFunc("/register", RegisterUserHandler(repo))

	http.HandleFunc("/login", LoginUserHandler(repo))

	http.HandleFunc("/login-google", LoginWGoogleHandler(repo))

	http.Handle("/get-me", AuthMiddleware(getMeHandler(repo), repo))

	http.Handle("/update-user", AuthMiddleware(updateUserHandler(repo), repo))

	http.Handle("/create-group", AuthMiddleware(CreateGroupHandler(repo), repo))

	http.Handle("/groups", AuthMiddleware(GetGroups(repo), repo))

	http.Handle("/send-add-group-request", AuthMiddleware(SendAddRequest(repo), repo))

	http.Handle("/get-my-add-requests", AuthMiddleware(handleGetAddRequests(repo), repo))

	http.Handle("/accept-add-request", AuthMiddleware(handleAcceptAddRequest(repo), repo))

	http.Handle("/reject-add-request", AuthMiddleware(handleRejectAddRequest(repo), repo))

	http.Handle("/add-group-expense", AuthMiddleware(handleCreateGroupExpense(repo), repo))

	http.Handle("/pay-group-expense", AuthMiddleware(handlePayGroupExpense(repo), repo))

	http.Handle("/save-fcm-token", AuthMiddleware(handleSaveFCMToken(repo), repo))

	http.Handle("/add-group-token", AuthMiddleware(addGroupWithTokenHandler(repo), repo))

	http.Handle("/delete-expense", AuthMiddleware(handleDeleteExpense(repo), repo))

	http.Handle("/delete-account", AuthMiddleware(handleDeleteAccount(repo), repo))

	fs := http.FileServer(http.Dir("./uploads"))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", fs))
	http.Handle("/upload-photo", AuthMiddleware(uploadPhotoHandler(repo), repo))

	fmt.Println("🚀 Sunucu 80 portunda başlatıldı...")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

// Basit SQL ayırıcı (noktalı virgülle ayırır)
func splitSQL(sql string) []string {
	var result []string
	queries := ""
	for _, line := range splitLines(sql) {
		if line == "" || line[0] == '-' { // yorum satırlarını atla
			continue
		}
		queries += line + "\n"
		if line[len(line)-1] == ';' {
			result = append(result, queries)
			queries = ""
		}
	}
	return result
}

func splitLines(sql string) []string {
	var lines []string
	curr := ""
	for _, ch := range sql {
		if ch == '\n' {
			lines = append(lines, curr)
			curr = ""
		} else {
			curr += string(ch)
		}
	}
	if curr != "" {
		lines = append(lines, curr)
	}
	return lines
}
