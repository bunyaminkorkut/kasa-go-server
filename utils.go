package main

import (
	"errors"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func generateJWT(data map[string]string) (string, error) {
	claims := jwt.MapClaims{}

	// Kullanıcıdan gelen tüm verileri claim’e ekle
	for k, v := range data {
		claims[k] = v
	}

	claims["iss"] = "kasa-go-server"

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func decodeJWTWithoutValidation(tokenStr string) (map[string]interface{}, error) {
	parser := &jwt.Parser{}
	token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}
	return nil, errors.New("invalid claims")
}
