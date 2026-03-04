package api

import (
	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/pkg/jwt"
)

type BotServer struct {
	sessionService *session.Service
	jwtManager     *jwt.JWTManager
	tokenCache     *cache.TokenCache
}

func NewBotServer(sessionService *session.Service, jwtManager *jwt.JWTManager, tokenCache *cache.TokenCache) *BotServer {
	return &BotServer{
		sessionService: sessionService,
		jwtManager:     jwtManager,
		tokenCache:     tokenCache,
	}
}
