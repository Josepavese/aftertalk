# Research: Aftertalk Core

**Feature**: 001-aftertalk-core  
**Date**: 2026-03-04  
**Purpose**: Document technology decisions and best practices for Go implementation

## Technology Stack Decisions

### 1. Language Selection: Go vs Node.js/Python

**Decision**: Go 1.22+

**Rationale**:
- **Performance native**: Goroutines gestiscono 10K+ connessioni concorrenti con ~2KB stack per goroutine
- **Pion WebRTC**: Libreria WebRTC pure-Go, matura, attivamente mantenuta, superior a Node.js alternatives
- **Single binary deployment**: 15MB binary vs 300MB+ container multi-service
- **Memory efficiency**: 50MB baseline vs 300MB+ Node.js/Python
- **Startup time**: 10ms vs 1-3s runtime initialization
- **Type safety**: Compile-time checks, nessun runtime type error
- **Standard library**: HTTP server, JSON encoding, crypto built-in
- **Cross-compilation**: `GOOS=linux go build` - nessuna dipendenza target platform
- **Cost efficiency**: 75% riduzione costi cloud per stesso carico

**Alternatives Considered**:
- **Node.js + Python**: 3-service architecture, communication overhead, 4x higher costs
- **Rust**: Superior performance, but steep learning curve and lower development velocity
- **Full Node.js**: Technological consistency, but lower performance for WebRTC and AI processing

**Best Practices**:
- Use Go modules for dependency management
- Implement graceful shutdown with context propagation
- Use structured logging (zap/zerolog)
- Profile with pprof before optimization
- Use internal/ for encapsulation, pkg/ for reusable code

### 2. WebRTC Library: Pion

**Decision**: `pion/webrtc` v4+

**Rationale**:
- Pure Go implementation (no CGo dependencies)
- Active maintenance (1K+ GitHub stars, regular releases)
- Comprehensive WebRTC support (ICE, DTLS, SRTP, SCTP)
- Idiomatic Go API
- Excellent documentation and examples
- Used in production by multiple companies

**Alternatives Considered**:
- **node-datachannel**: Node.js binding, requires native dependencies
- **werift**: TypeScript implementation, less mature
- **Go-WebRTC**: Google's implementation, unmaintained

**Best Practices**:
```go
// PeerConnection configuration
config := webrtc.Configuration{
    ICEServers: []webrtc.ICEServer{
        {URLs: []string{"stun:stun.l.google.com:19302"}},
    },
}

// Track handling with goroutines
peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
    go func() {
        for {
            pkt, _, err := track.ReadRTP()
            if err != nil {
                return
            }
            // Process audio packet
            processAudio(pkt)
        }
    }()
})
```

### 3. HTTP Router: Chi vs Gin vs Standard Library

**Decision**: Chi (or standard library for minimal dependencies)

**Rationale**:
- Chi: Lightweight, idiomatic, composable middleware
- Standard library: Zero dependencies, built-in context support
- Gin: Faster, but custom context type (less idiomatic)

**Alternatives Considered**:
- **Gin**: Superior performance, but less idiomatic
- **Echo**: Good, but more features than necessary
- **Fiber**: Express-like API, but does not use standard library types

**Best Practices**:
```go
// Chi router setup
r := chi.NewRouter()
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)
r.Use(middleware.Timeout(60 * time.Second))

r.Route("/v1", func(r chi.Router) {
    r.Use(authMiddleware)
    r.Post("/sessions", h.CreateSession)
    r.Get("/sessions/{id}", h.GetSession)
    r.Get("/sessions/{id}/minutes", h.GetMinutes)
})

// Standard library alternative
mux := http.NewServeMux()
mux.HandleFunc("POST /v1/sessions", h.CreateSession)
mux.HandleFunc("GET /v1/sessions/{id}", h.GetSession)
```

### 4. Database Driver: pgx vs lib/pq

**Decision**: `pgx` v5

**Rationale**:
- Pure Go, no CGo
- Better performance (connection pool, prepared statements)
- Native PostgreSQL features (LISTEN/NOTIFY, COPY)
- Type-safe query building (optional)
- Active maintenance

**Alternatives Considered**:
- **lib/pq**: Stable, but less performant
- **GORM**: ORM, but abstraction layer not necessary
- **sqlc**: Type-safe queries, but additional build step

**Best Practices**:
```go
// Connection pool
config, _ := pgxpool.ParseConfig(databaseURL)
config.MaxConns = 100
config.MinConns = 10
config.MaxConnLifetime = 1 * time.Hour
config.MaxConnIdleTime = 10 * time.Minute

pool, _ := pgxpool.NewWithConfig(ctx, config)

// Prepared statements
stmt, _ := pool.Prepare(ctx, "get_session", "SELECT * FROM sessions WHERE id = $1")
row := pool.QueryRow(ctx, "get_session", sessionID)

// Batch operations
batch := &pgx.Batch{}
batch.Queue("INSERT INTO transcriptions (...) VALUES (...)")
batch.Queue("UPDATE sessions SET status = $1", "processing")
results := pool.SendBatch(ctx, batch)
defer results.Close()
```

### 5. STT Provider: HTTP Client Implementation

**Decision**: Custom HTTP client with provider interface

**Rationale**:
- Go HTTP client is excellent (connection pooling, timeouts, retries)
- No need for Python SDKs when APIs are REST-based
- Type-safe request/response structs
- Full control over retry logic and error handling

**Alternatives Considered**:
- **Google Cloud Go SDK**: Official, but heavy dependencies
- **AWS SDK for Go**: Good, but provider lock-in

**Best Practices**:
```go
type STTProvider interface {
    Transcribe(ctx context.Context, audio []byte, config STTConfig) (*Transcription, error)
}

type GoogleSTT struct {
    client *http.Client
    apiKey string
}

func (g *GoogleSTT) Transcribe(ctx context.Context, audio []byte, config STTConfig) (*Transcription, error) {
    req := GoogleSTTRequest{
        Audio: base64.StdEncoding.EncodeToString(audio),
        Config: GoogleSTTConfig{
            LanguageCode: config.Language,
            Encoding:     "LINEAR16",
            SampleRate:   16000,
        },
    }
    
    var resp GoogleSTTResponse
    err := g.doRequest(ctx, "POST", g.apiURL, req, &resp)
    if err != nil {
        return nil, fmt.Errorf("google stt: %w", err)
    }
    
    return g.parseResponse(resp), nil
}

// Retry with exponential backoff
func (g *GoogleSTT) doRequest(ctx context.Context, method, url string, req, resp interface{}) error {
    return retry.Do(func() error {
        body, _ := json.Marshal(req)
        httpReq, _ := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
        
        httpResp, err := g.client.Do(httpReq)
        if err != nil {
            return retry.Unrecoverable(err)
        }
        defer httpResp.Body.Close()
        
        if httpResp.StatusCode >= 500 {
            return fmt.Errorf("server error: %d", httpResp.StatusCode)
        }
        
        return json.NewDecoder(httpResp.Body).Decode(resp)
    }, retry.Attempts(3), retry.Delay(1*time.Second))
}
```

### 6. LLM Provider: HTTP Client Implementation

**Decision**: Custom HTTP client with provider interface

**Rationale**:
- Same as STT: Go HTTP client is sufficient
- Type-safe structs for requests/responses
- Control over token counting and cost optimization

**Best Practices**:
```go
type LLMProvider interface {
    Generate(ctx context.Context, prompt string, config LLMConfig) (*Response, error)
}

type OpenAI struct {
    client *http.Client
    apiKey string
    model  string
}

func (o *OpenAI) Generate(ctx context.Context, prompt string, config LLMConfig) (*Response, error) {
    req := OpenAIRequest{
        Model: o.model,
        Messages: []Message{
            {Role: "system", Content: config.SystemPrompt},
            {Role: "user", Content: prompt},
        },
        Temperature:   config.Temperature,
        MaxTokens:     config.MaxTokens,
        ResponseFormat: map[string]string{"type": "json_object"},
    }
    
    var resp OpenAIResponse
    err := o.doRequest(ctx, req, &resp)
    if err != nil {
        return nil, fmt.Errorf("openai: %w", err)
    }
    
    return &Response{
        Content: resp.Choices[0].Message.Content,
        Tokens:  resp.Usage.TotalTokens,
    }, nil
}
```

### 7. Configuration Management: Koanf

**Decision**: `knadh/koanf`

**Rationale**:
- Unified configuration loading (file, env, flags)
- Type-safe access
- No reflection overhead
- Lightweight

**Alternatives Considered**:
- **Viper**: Popular, but heavy reflection usage
- **Envconfig**: Simple, but env-only
- **Standard library**: Manual parsing

**Best Practices**:
```go
type Config struct {
    HTTPPort    int    `koanf:"http_port"`
    WSPort      int    `koanf:"ws_port"`
    DatabaseURL string `koanf:"database_url"`
    RedisURL    string `koanf:"redis_url"`
    STTProvider string `koanf:"stt_provider"`
    STTAPIKey   string `koanf:"stt_api_key"`
    LLMProvider string `koanf:"llm_provider"`
    LLMAPIKey   string `koanf:"llm_api_key"`
}

func LoadConfig() (*Config, error) {
    k := koanf.New(".")
    
    // Load defaults
    k.Load(structs.Provider(Config{
        HTTPPort:    8080,
        WSPort:      8081,
        STTProvider: "google",
        LLMProvider: "openai",
    }), nil)
    
    // Load env vars
    k.Load(env.Provider("", ".", func(s string) string {
        return strings.ToLower(strings.ReplaceAll(s, "_", "."))
    }), nil)
    
    // Load config file
    if _, err := os.Stat("config.yaml"); err == nil {
        k.Load(file.Provider("config.yaml"), yaml.Parser())
    }
    
    var cfg Config
    k.Unmarshal("", &cfg)
    return &cfg, nil
}
```

### 8. Logging: Zap vs Zerolog

**Decision**: `uber-go/zap`

**Rationale**:
- Structured logging
- High performance
- Type-safe field API
- Industry standard

**Alternatives Considered**:
- **Zerolog**: Faster, but fewer features
- **Logrus**: Popular, but slower
- **Standard library**: Unstructured

**Best Practices**:
```go
logger, _ := zap.NewProduction()
defer logger.Sync()

// Structured logging
logger.Info("session created",
    zap.String("session_id", sessionID),
    zap.String("user_id", userID),
    zap.Duration("duration", time.Since(start)),
)

// Request-scoped logger
func loggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Create request-scoped logger
            reqLogger := logger.With(
                zap.String("request_id", r.Header.Get("X-Request-ID")),
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
            )
            
            // Add to context
            ctx := context.WithValue(r.Context(), "logger", reqLogger)
            next.ServeHTTP(w, r.WithContext(ctx))
            
            reqLogger.Info("request completed",
                zap.Duration("duration", time.Since(start)),
                zap.Int("status", w.(*responseWriter).status),
            )
        })
    }
}
```

### 9. Testing Strategy

**Decision**: Standard testing package + testify + mockery

**Rationale**:
- Standard library: Built-in, no dependencies
- Testify: Assertions, mocking, suite support
- Mockery: Mock generation from interfaces

**Best Practices**:
```go
// Unit test
func TestTranscribe(t *testing.T) {
    mockSTT := &MockSTTProvider{
        TranscribeFunc: func(ctx context.Context, audio []byte, cfg STTConfig) (*Transcription, error) {
            return &Transcription{Segments: []Segment{{Text: "test"}}}, nil
        },
    }
    
    pipeline := ai.NewPipeline(mockSTT, nil)
    
    result, err := pipeline.Transcribe(context.Background(), []byte("audio"))
    
    require.NoError(t, err)
    assert.Len(t, result.Segments, 1)
}

// Integration test
func TestFullPipeline(t *testing.T) {
    if testing.Short() {
        t.Skip("integration test")
    }
    
    // Setup test database
    db := setupTestDB(t)
    defer db.Close()
    
    // Start server
    server := httptest.NewServer(setupRouter(db))
    defer server.Close()
    
    // Test flow
    resp, err := http.Post(server.URL+"/v1/sessions", "application/json", 
        strings.NewReader(`{"participants":[...]}`))
    require.NoError(t, err)
    
    var session Session
    json.NewDecoder(resp.Body).Decode(&session)
    
    assert.NotEmpty(t, session.ID)
}

// Table-driven test
func TestValidateToken(t *testing.T) {
    tests := []struct {
        name    string
        token   string
        wantErr bool
    }{
        {"valid", "valid-token", false},
        {"expired", "expired-token", true},
        {"invalid", "invalid-token", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateToken(tt.token)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 10. Error Handling

**Decision**: Custom error types with wrapping

**Rationale**:
- Idiomatic Go error handling
- Stack trace preservation
- Type assertion for specific error handling

**Best Practices**:
```go
// Custom error types
type TranscriptionError struct {
    SessionID string
    Provider  string
    Err       error
}

func (e *TranscriptionError) Error() string {
    return fmt.Sprintf("transcription failed for session %s: %v", e.SessionID, e.Err)
}

func (e *TranscriptionError) Unwrap() error {
    return e.Err
}

// Error wrapping
func (s *Service) ProcessSession(ctx context.Context, sessionID string) error {
    transcription, err := s.Transcribe(ctx, sessionID)
    if err != nil {
        return fmt.Errorf("process session %s: %w", sessionID, err)
    }
    
    minutes, err := s.GenerateMinutes(ctx, transcription)
    if err != nil {
        return fmt.Errorf("process session %s: %w", sessionID, err)
    }
    
    return nil
}

// Error type assertion
var transErr *TranscriptionError
if errors.As(err, &transErr) {
    // Handle transcription-specific error
    log.Error("transcription failed", 
        zap.String("session", transErr.SessionID),
        zap.String("provider", transErr.Provider),
    )
}
```

## Performance Optimizations

### 1. Connection Pooling

```go
// HTTP client with connection pooling
var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
    Timeout: 30 * time.Second,
}
```

### 2. Goroutine Pool

```go
// Worker pool for bounded concurrency
type Pool struct {
    tasks   chan Task
    workers int
}

func NewPool(workers int) *Pool {
    return &Pool{
        tasks:   make(chan Task, workers*2),
        workers: workers,
    }
}

func (p *Pool) Start(ctx context.Context, process func(Task) error) {
    var wg sync.WaitGroup
    
    for i := 0; i < p.workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case task := <-p.tasks:
                    if err := process(task); err != nil {
                        log.Error("task failed", zap.Error(err))
                    }
                case <-ctx.Done():
                    return
                }
            }
        }()
    }
    
    wg.Wait()
}
```

### 3. Memory Pool

```go
// Sync.Pool for reusable buffers
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func processAudio(audio []byte) {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()
    
    buf.Write(audio)
    // Process buffer...
}
```

## Monitoring & Observability

### Metrics (Prometheus)

```go
var (
    sessionsCreated = promauto.NewCounter(prometheus.CounterOpts{
        Name: "aftertalk_sessions_created_total",
        Help: "Total sessions created",
    })
    
    processingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "aftertalk_processing_duration_seconds",
        Help:    "Session processing duration",
        Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
    })
)

// Usage
func (s *Service) ProcessSession(sessionID string) error {
    start := time.Now()
    defer func() {
        processingDuration.Observe(time.Since(start).Seconds())
    }()
    
    sessionsCreated.Inc()
    // ...
}
```

### Tracing (OpenTelemetry)

```go
func (s *Service) ProcessSession(ctx context.Context, sessionID string) error {
    ctx, span := tracer.Start(ctx, "ProcessSession")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("session.id", sessionID),
    )
    
    // Child spans automatically linked
    transcription, err := s.Transcribe(ctx, sessionID)
    // ...
}
```

## Security Considerations

### 1. JWT Validation

```go
func validateToken(tokenString string) (*Claims, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return publicKey, nil
    })
    
    if err != nil {
        return nil, err
    }
    
    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok || !token.Valid {
        return nil, errors.New("invalid token")
    }
    
    return &Claims{
        SessionID: claims["callId"].(string),
        UserID:    claims["userId"].(string),
        Role:      claims["role"].(string),
        JTI:       claims["jti"].(string),
    }, nil
}
```

### 2. Input Validation

```go
type CreateSessionRequest struct {
    Participants []Participant `json:"participants"`
}

func (r *CreateSessionRequest) Validate() error {
    if len(r.Participants) < 2 {
        return errors.New("at least 2 participants required")
    }
    
    for _, p := range r.Participants {
        if p.UserID == "" {
            return errors.New("user_id required")
        }
        if p.Role == "" {
            return errors.New("role required")
        }
    }
    
    return nil
}
```

## Open Questions

1. **Local Whisper integration**: Should we support local Whisper model for privacy/cost? (Defer: evaluate costs vs cloud STT)
2. **Redis necessity**: Is Redis needed if in-process cache handles hot data? (Keep for distributed deployments)
3. **gRPC for future**: Should API support gRPC for better performance? (Defer: HTTP is sufficient for MVP)

## Next Steps

1. Implement core data structures and interfaces
2. Build HTTP API with Chi router
3. Implement Bot Recorder with Pion
4. Create STT/LLM provider adapters
5. Add Prometheus metrics
6. Write comprehensive tests
