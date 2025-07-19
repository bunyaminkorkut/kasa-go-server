package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func main() {
	// .env dosyasını yükle
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// DB bağlantı bilgilerini al (dbname çıkarıldı)
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASS")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, pass, host, port)

	// DB'ye bağlan
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// SQL scriptini oku
	sqlBytes, err := ioutil.ReadFile("./database/schema.sql")
	if err != nil {
		log.Fatal(err)
	}
	sqlScript := string(sqlBytes)

	// SQL scriptini çalıştır
	_, err = db.Exec(sqlScript)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("SQL script başarıyla çalıştırıldı.")

	// HTTP endpointleri tanımla
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Merhaba, Go server çalışıyor!")
	})

	http.HandleFunc("/v", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Merhaba, test serverı çalışıyor!")
	})

	fmt.Println("Sunucu 80 portunda başlatıldı...")
	log.Fatal(http.ListenAndServe(":80", nil))
}
