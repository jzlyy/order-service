package utils

import (
	"github.com/golang-jwt/jwt/v5"
	"order-service/config"
)

func ParseToken(tokenString string) (int, error) {
	cfg := config.LoadConfig()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return 0, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		userID := int(claims["user_id"].(float64))
		return userID, nil
	}

	return 0, err
}
