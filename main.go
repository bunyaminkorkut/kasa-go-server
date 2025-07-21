package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

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
	firebaseAuth, err := connectToFirebase(context.Background())
	if err != nil {
		log.Fatal("❌ Firebase bağlantısı başarısız:", err)
	}
	FirebaseAuth = firebaseAuth // middleware erişebilsin diye global değişkene ata

	// Sunucu başlatma
	repo := &KasaRepository{DB: db}

	// HTTP endpointleri
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Aşkımmm")
	})

	http.HandleFunc("/v", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Merhaba, test serverı çalışıyor!")
	})
	http.HandleFunc("/register", RegisterUserHandler(repo))

	http.HandleFunc("/login", LoginUserHandler(repo))

	http.Handle("/create-group", AuthMiddleware(CreateGroupHandler(repo), repo))

	http.Handle("/groups", AuthMiddleware(GetGroups(repo), repo))

	http.Handle("/send-add-group-request", AuthMiddleware(SendAddRequest(repo), repo))

	http.Handle("/get-my-add-requests", AuthMiddleware(handleGetAddRequests(repo), repo))

	http.Handle("/accept-add-request", AuthMiddleware(handleAcceptAddRequest(repo), repo))

	http.Handle("/reject-add-request", AuthMiddleware(handleRejectAddRequest(repo), repo))

	fmt.Println("🚀 Sunucu 80 portunda başlatıldı...")
	log.Fatal(http.ListenAndServe(":80", nil))
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
