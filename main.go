package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {

	// Ana endpoint
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Merhaba, Go server çalışıyor!")
	})

	http.HandleFunc("/deneme", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Merhaba, deneme serverı çalışıyor!")
	})

	fmt.Println("Sunucu 8080 portunda başlatıldı...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
