package main

import (
	"context" 
	"fmt"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string
const userContextKey = contextKey("userID")

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			if err == http.ErrNoCookie {
				http.Error(w, "인증이 필요합니다.", http.StatusUnauthorized)
				return
			}
			http.Error(w, "잘못된 요청입니다.", http.StatusBadRequest)
			return
		}

		tokenString := cookie.Value
		claims := &jwtClaims{} 
		jwtKey := []byte(os.Getenv("JWT_SECRET_KEY"))

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("예상치 못한 서명 알고리즘: %v", token.Header["alg"])
			}
			return jwtKey, nil 
		})

		if err != nil || !token.Valid {
			http.Error(w, "인증이 유효하지 않습니다.", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, claims.UserID)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}