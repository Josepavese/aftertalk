# Aftertalk - Dependencies

All dependencies are kept at the **latest stable versions** and updated regularly.

## Core Dependencies

### WebRTC & Audio
- **pion/webrtc v4.2.9** - Latest WebRTC implementation in pure Go
  - Pure Go, no CGO required
  - Production-ready with active maintenance
  - Full WebRTC support (PeerConnection, DataChannels, etc.)

### Database
- **modernc.org/sqlite v1.46.1** - Pure Go SQLite driver
  - Zero CGO dependencies
  - Full SQLite 3 support
  - WAL mode for concurrent access

### HTTP & API
- **go-chi/chi v5** - Lightweight, idiomatic HTTP router
- **go-chi/cors** - CORS middleware

### Configuration
- **knadh/koanf v2.3.2** - Lightweight configuration management
  - Environment variables support
  - YAML file support
  - No external dependencies

### Logging
- **uber-go/zap v1.27.1** - Blazing fast, structured logging
  - Zero-allocation JSON logging
  - Production-ready

### Security
- **golang-jwt/jwt v5.3.1** - JWT implementation
- **google/uuid v1.6.0** - UUID generation

## Version Strategy

✅ **Always use latest stable versions**
✅ **Regular updates with `go get -u all`**
✅ **No legacy/deprecated packages**
✅ **Security fixes applied immediately**

## Update Commands

```bash
# Update all dependencies
go get -u all

# Update specific package
go get -u github.com/pion/webrtc/v4@latest

# Tidy and verify
go mod tidy && go mod verify
```

## Dependency Policy

1. **Latest versions**: All dependencies are at the latest stable release
2. **Security first**: Immediate updates for security vulnerabilities
3. **No CGO**: Pure Go implementations where possible (SQLite, WebRTC)
4. **Minimal dependencies**: Only essential packages included
5. **Active maintenance**: All dependencies have active maintainers

Last updated: 2026-03-04
