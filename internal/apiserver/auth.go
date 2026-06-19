package apiserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const authUserContextKey = "auth_user"

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Tenant   string `json:"tenant"`
	Region   string `json:"region"`
}

type userPayload struct {
	Username string `json:"username"`
	Tenant   string `json:"tenant"`
	Region   string `json:"region"`
}

type loginPayload struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      userPayload `json:"user"`
}

type authClaims struct {
	Username string `json:"username"`
	Tenant   string `json:"tenant"`
	Region   string `json:"region"`
	Expires  int64  `json:"expires"`
}

func (s *Service) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.writeError(c, http.StatusBadRequest, err)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Tenant = strings.TrimSpace(req.Tenant)
	req.Region = strings.TrimSpace(req.Region)
	if req.Tenant == "" {
		req.Tenant = "local-demo"
	}
	if req.Region == "" {
		req.Region = "local"
	}

	if !s.credentialsMatch(req.Username, req.Password) {
		s.writeError(c, http.StatusUnauthorized, errors.New("invalid username or password"))
		return
	}

	expiresAt := time.Now().UTC().Add(s.cfg.AuthSessionTTL)
	token, err := s.signAuthToken(authClaims{
		Username: req.Username,
		Tenant:   req.Tenant,
		Region:   req.Region,
		Expires:  expiresAt.Unix(),
	})
	if err != nil {
		s.writeError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, loginPayload{
		Token:     token,
		ExpiresAt: expiresAt,
		User: userPayload{
			Username: req.Username,
			Tenant:   req.Tenant,
			Region:   req.Region,
		},
	})
}

func (s *Service) me(c *gin.Context) {
	claims := authClaims{}
	if value, exists := c.Get(authUserContextKey); exists {
		if parsed, ok := value.(authClaims); ok {
			claims = parsed
		}
	}
	if claims.Username == "" {
		claims = authClaims{
			Username: s.cfg.AuthUsername,
			Tenant:   "local-demo",
			Region:   "local",
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"user": userPayload{
			Username: claims.Username,
			Tenant:   claims.Tenant,
			Region:   claims.Region,
		},
	})
}

func (s *Service) credentialsMatch(username, password string) bool {
	if subtle.ConstantTimeCompare([]byte(username), []byte(s.cfg.AuthUsername)) != 1 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.AuthPassword)) == 1
}

func (s *Service) signAuthToken(claims authClaims) (string, error) {
	if strings.TrimSpace(claims.Username) == "" {
		return "", errors.New("username is required")
	}
	if strings.TrimSpace(claims.Tenant) == "" {
		claims.Tenant = "local-demo"
	}
	if strings.TrimSpace(claims.Region) == "" {
		claims.Region = "local"
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signature := s.authSignature(string(payload))
	return base64.RawURLEncoding.EncodeToString(payload) + "." + signature, nil
}

func (s *Service) parseAuthToken(token string, now time.Time) (authClaims, error) {
	payloadPart, signature, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || payloadPart == "" || signature == "" {
		return authClaims{}, errors.New("invalid auth token")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return authClaims{}, errors.New("invalid auth token")
	}

	expectedSignature := s.authSignature(string(decoded))
	if subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSignature)) != 1 {
		return authClaims{}, errors.New("invalid auth token")
	}

	var claims authClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return authClaims{}, errors.New("invalid auth token")
	}
	if strings.TrimSpace(claims.Username) == "" {
		return authClaims{}, errors.New("invalid auth token")
	}
	if now.UTC().Unix() >= claims.Expires {
		return authClaims{}, errors.New("auth token expired")
	}
	return claims, nil
}

func (s *Service) authSignature(payload string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.AuthTokenSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Service) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cfg.AuthEnabled {
			c.Next()
			return
		}

		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			s.writeError(c, http.StatusUnauthorized, errors.New("authorization bearer token required"))
			c.Abort()
			return
		}
		token := strings.TrimSpace(header[len("Bearer "):])
		claims, err := s.parseAuthToken(token, time.Now().UTC())
		if err != nil {
			s.writeError(c, http.StatusUnauthorized, err)
			c.Abort()
			return
		}
		c.Set(authUserContextKey, claims)
		c.Next()
	}
}
