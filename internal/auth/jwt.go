package auth

import (
	"errors"
	"time"

	"github.com/eventpulse/backend/config"
	"github.com/eventpulse/backend/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UsuarioID string     `json:"usuario_id"`
	EventoID  *string    `json:"evento_id"`
	Rol       models.Rol `json:"rol"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret     []byte
	expiration time.Duration
}

func NewJWTService(cfg *config.Config) *JWTService {
	return &JWTService{
		secret:     []byte(cfg.JWT.Secret),
		expiration: time.Duration(cfg.JWT.ExpirationHours) * time.Hour,
	}
}

func (j *JWTService) GenerarToken(usuario *models.Usuario) (string, error) {
	claims := Claims{
		UsuarioID: usuario.ID,
		EventoID:  usuario.EventoID,
		Rol:       usuario.Rol,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   usuario.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func (j *JWTService) ValidarToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("método de firma inesperado")
		}
		return j.secret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("token inválido")
	}

	return claims, nil
}
