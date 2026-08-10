package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/auth"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/handlers"
	"msoffice2pdf/internal/repo"
)

type contextKey string

const currentUserKey contextKey = "currentUser"

const (
	codeUnauthorized = 40101
	codeForbidden    = 40301
)

func CurrentUser(c *gin.Context) (*domain.User, bool) {
	v, ok := c.Get(string(currentUserKey))
	if !ok {
		return nil, false
	}
	u, ok := v.(*domain.User)
	return u, ok
}

func setCurrentUser(c *gin.Context, u *domain.User) {
	c.Set(string(currentUserKey), u)
	if u != nil && u.UID != "" {
		ctx := applog.ContextWithUID(c.Request.Context(), u.UID)
		c.Request = c.Request.WithContext(ctx)
	}
}

func AuthRequired(jwtSecret string, users *repo.UserRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := strings.TrimSpace(c.GetHeader("X-UID"))
		apiToken := strings.TrimSpace(c.GetHeader("X-Token"))

		if uid != "" && apiToken != "" {
			if authenticateAPIToken(c, users, uid, apiToken) {
				c.Next()
			}
			return
		}

		if jwtStr := extractJWT(c); jwtStr != "" {
			if authenticateJWT(c, users, jwtSecret, jwtStr) {
				c.Next()
			}
			return
		}

		handlers.Fail(c, http.StatusUnauthorized, codeUnauthorized, "unauthorized")
		c.Abort()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := CurrentUser(c)
		if !ok || u.Role != domain.RoleAdmin {
			handlers.Fail(c, http.StatusForbidden, codeForbidden, "forbidden")
			c.Abort()
			return
		}
		c.Next()
	}
}

func authenticateAPIToken(c *gin.Context, users *repo.UserRepo, uid, apiToken string) bool {
	u, err := users.FindByUID(uid)
	if err != nil {
		handlers.Fail(c, http.StatusUnauthorized, codeUnauthorized, "unauthorized")
		c.Abort()
		return false
	}
	if u == nil || !tokensMatch(u.Token, apiToken) {
		handlers.Fail(c, http.StatusUnauthorized, codeUnauthorized, "unauthorized")
		c.Abort()
		return false
	}
	if u.Status == domain.StatusFrozen {
		handlers.Fail(c, http.StatusForbidden, codeForbidden, "account frozen")
		c.Abort()
		return false
	}
	setCurrentUser(c, u)
	return true
}

func authenticateJWT(c *gin.Context, users *repo.UserRepo, jwtSecret, jwtStr string) bool {
	claims, err := auth.ParseJWT(jwtStr, jwtSecret)
	if err != nil {
		handlers.Fail(c, http.StatusUnauthorized, codeUnauthorized, "unauthorized")
		c.Abort()
		return false
	}

	u, err := users.FindByUID(claims.UID)
	if err != nil || u == nil {
		handlers.Fail(c, http.StatusUnauthorized, codeUnauthorized, "unauthorized")
		c.Abort()
		return false
	}
	if u.Status == domain.StatusFrozen {
		handlers.Fail(c, http.StatusForbidden, codeForbidden, "account frozen")
		c.Abort()
		return false
	}
	setCurrentUser(c, u)
	return true
}

func extractJWT(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(authHeader, prefix) {
			return strings.TrimSpace(authHeader[len(prefix):])
		}
	}
	if cookie, err := c.Cookie("access_token"); err == nil {
		return strings.TrimSpace(cookie)
	}
	return ""
}

func tokensMatch(stored, provided string) bool {
	if len(stored) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(provided)) == 1
}
