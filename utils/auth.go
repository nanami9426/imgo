package utils

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strconv"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var node, _ = snowflake.NewNode(1)

func JWTSecret() []byte {
	return []byte(DefaultJWTSecret)
}

func JWTTTL() time.Duration {
	return DefaultJWTTTL
}

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func CheckPassword(hashed string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
	return err == nil
}

func GenerateID() int64 {
	user_id := node.Generate().Int64()
	return user_id
}

func GenerateUserID() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(900000000))
	return n.Int64() + 100000000
}

type Claims struct {
	UserID  uint   `json:"user_id"`
	Role    string `json:"role"`
	Version uint
	jwt.RegisteredClaims
}

func GenerateToken(secret []byte, user_id uint, role string, ttl time.Duration, version uint) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:  user_id,
		Role:    role,
		Version: version,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(user_id), 10), // 这个token代表的主体
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now), // 在这个时间之前token不允许被使用
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(secret)
}

func CheckToken(tokenString string, secret []byte) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("empty token")
	}
	if len(secret) == 0 {
		return nil, errors.New("empty secret")
	}
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func GetTokenVersion(ctx context.Context, user_id uint) (uint, error) {
	key := strconv.FormatUint(uint64(user_id), 10)

	versionStr, err := RDB.Get(ctx, key).Result()
	if err == redis.Nil {
		// key 不存在，返回默认版本
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	version, err := strconv.ParseUint(versionStr, 10, 0)
	if err != nil {
		return 0, err
	}

	return uint(version), nil
}

func IncrTokenVersion(ctx context.Context, userID uint) (uint, error) {
	key := strconv.FormatUint(uint64(userID), 10)

	// Redis INCR：不存在的 key 会被当成 0，然后 +1
	newVersion, err := RDB.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	return uint(newVersion), nil
}
