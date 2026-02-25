package middleware

import (
	"net/http"
	"strings"

	"github.com/eventpulse/backend/internal/auth"
	"github.com/eventpulse/backend/internal/models"
	"github.com/gin-gonic/gin"
)

const (
	CtxUsuarioID = "usuario_id"
	CtxEventoID  = "evento_id"
	CtxRol       = "rol"
)

func Auth(jwtSvc *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Token requerido"})
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Formato inválido: Bearer <token>"})
			return
		}
		claims, err := jwtSvc.ValidarToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Token inválido o expirado"})
			return
		}
		c.Set(CtxUsuarioID, claims.UsuarioID)
		c.Set(CtxEventoID, claims.EventoID)
		c.Set(CtxRol, string(claims.Rol))
		c.Next()
	}
}

// SoloAdmin restringe el acceso solo al rol admin
func SoloAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString(CtxRol) != string(models.RolAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, models.ErrorResponse{Error: "Solo el admin puede realizar esta acción"})
			return
		}
		c.Next()
	}
}

// SoloTrabajador permite cualquier rol excepto acciones reservadas al admin
func RequiereEventoActivo() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString(CtxEventoID) == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, models.ErrorResponse{Error: "No estás vinculado a ningún evento activo"})
			return
		}
		c.Next()
	}
}

func GetUsuarioID(c *gin.Context) string { return c.GetString(CtxUsuarioID) }
func GetEventoID(c *gin.Context) string  { return c.GetString(CtxEventoID) }
func GetRol(c *gin.Context) models.Rol   { return models.Rol(c.GetString(CtxRol)) }
