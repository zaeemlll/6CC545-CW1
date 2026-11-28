package security

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type TokenValidator struct {
	Key              []byte
	Issuer, Audience string
	Now              func() time.Time
}

const maxBearerLength = 8 * 1024

func (v TokenValidator) Validate(raw, requiredRole string) (Claims, error) {
	var claims Claims

	if len(raw) == 0 || len(raw) > maxBearerLength {
		return claims, ErrUnauthenticated
	}

	if len(v.Key) < 32 || v.Issuer == "" || v.Audience == "" {
		return claims, ErrUnauthenticated
	}

	if requiredRole != "reader" && requiredRole != "writer" {
		return claims, ErrForbidden
	}

	now := v.Now
	if now == nil {
		now = time.Now
	}

	parsed, err := jwt.ParseWithClaims(
		raw,
		&claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrUnauthenticated
			}
			return v.Key, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(v.Issuer),
		jwt.WithAudience(v.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithNotBeforeRequired(),
		jwt.WithTimeFunc(now),
	)
	if err != nil || parsed == nil || !parsed.Valid {
		return Claims{}, ErrUnauthenticated
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return Claims{}, ErrUnauthenticated
	}

	if claims.Role != "reader" && claims.Role != "writer" {
		return Claims{}, ErrUnauthenticated
	}

	if requiredRole == "writer" && claims.Role != "writer" {
		return Claims{}, ErrForbidden
	}

	return claims, nil
}

func Require(v TokenValidator, requiredRole string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", "Bearer")

		values := r.Header.Values("Authorization")
		if len(values) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		header := values[0]
		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		raw := strings.TrimPrefix(header, "Bearer ")
		if raw == "" || strings.ContainsAny(raw, " \t\r\n") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := v.Validate(raw, requiredRole)
		if err != nil {
			if errors.Is(err, ErrForbidden) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), privateContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// IssueReaderToken creates a short-lived token for internal metadata reads.
func (v TokenValidator) IssueReaderToken(subject string, ttl time.Duration) (string, error) {
	if len(v.Key) < 32 || v.Issuer == "" || v.Audience == "" {
		return "", errors.New("invalid JWT signing configuration")
	}
	if strings.TrimSpace(subject) == "" {
		return "", errors.New("JWT subject is required")
	}
	if ttl <= 0 || ttl > time.Minute {
		return "", errors.New("reader token lifetime must be between 0 and 1 minute")
	}

	now := time.Now()
	if v.Now != nil {
		now = v.Now()
	}

	claims := Claims{
		Role: "reader",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    v.Issuer,
			Audience:  jwt.ClaimStrings{v.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(v.Key)
}
