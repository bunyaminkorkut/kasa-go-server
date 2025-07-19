package main

type UserRegisterRequest struct {
	FullName string `json:"fullname"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Iban     string `json:"iban"`
}
