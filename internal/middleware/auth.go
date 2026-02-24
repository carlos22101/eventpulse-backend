package middleware

import (
	"net/http"
	"strings"

	"github.com/eventpulse/backend/internal/auth"
	"github.com/eventpulse/backend/internal/models"
	"github.com/gin-gonic/gin"
)

const (
	ContextUsuarioID = "usuario_id"
	ContextEventoID  = "evento_id"
	ContextRol       = "rol"
)

func Auth(jwtSvc *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "Token requerido",
			})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "Formato de token inválido",
			})
			return
		}

		claims, err := jwtSvc.ValidarToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "Token inválido o expirado",
			})
			return
		}

		c.Set(ContextUsuarioID, claims.UsuarioID)
		c.Set(ContextEventoID, claims.EventoID)
		c.Set(ContextRol, string(claims.Rol))
		c.Next()
	}
}

// SoloSupervisor restringe endpoints a roles con permisos elevados
func SoloSupervisor() gin.HandlerFunc {
	return func(c *gin.Context) {
		rol := c.GetString(ContextRol)
		if rol != string(models.RolSupervisor) && rol != string(models.RolAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, models.ErrorResponse{
				Error: "Permisos insuficientes",
			})
			return
		}
		c.Next()
	}
}

// GetUsuarioID helper para extraer el ID del usuario del contexto
func GetUsuarioID(c *gin.Context) string {
	return c.GetString(ContextUsuarioID)
}

func GetEventoID(c *gin.Context) string {
	return c.GetString(ContextEventoID)
}
