return database.Transaction(func(tx \*gorm.DB) error {
// Deduct from sender
if err := tx.Model(&models.User{}).Where("id = ?", fromID).
Update("credits", gorm.Expr("credits - ?", amount)).Error; err != nil {
return err
}

        // Add to receiver
        if err := tx.Model(&models.User{}).Where("id = ?", toID).
            Update("credits", gorm.Expr("credits + ?", amount)).Error; err != nil {
            return err
        }

        return nil
    })

}

````

### MongoDB Operations
```go
// When using MongoDB
func (s *UserService) GetUserMongo(id string) (*models.UserMongo, error) {
    if !database.IsMongoDB() {
        return nil, errors.New("not using MongoDB")
    }

    objectID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        return nil, err
    }

    var user models.UserMongo
    err = s.db.MongoDB.Collection("users").FindOne(
        context.Background(),
        bson.M{"_id": objectID},
    ).Decode(&user)

    return &user, err
}
````

### Multi-Database Support Example

```go
// Switch databases by changing DB_DRIVER in .env
// DB_DRIVER=postgres
// DB_DRIVER=mysql
// DB_DRIVER=sqlite
// DB_DRIVER=mongodb

// The same code works with all databases
user := &models.User{
    Email: "user@example.com",
    Name:  "John Doe",
}

if err := db.Write.Create(user).Error; err != nil {
    return err
}
```

## Redis Caching

### Basic Cache Operations

```go
// In your service
func (s *UserService) GetUserWithCache(id uint) (*models.User, error) {
    // Try cache first
    cacheKey := fmt.Sprintf("user:%d", id)

    var user models.User
    if err := s.redis.CacheGetJSON("users", cacheKey, &user); err == nil {
        // Cache hit
        return &user, nil
    }

    // Cache miss - get from database
    if err := s.db.Read.First(&user, id).Error; err != nil {
        return nil, err
    }

    // Store in cache for 1 hour
    s.redis.CacheSet("users", cacheKey, &user, time.Hour)

    return &user, nil
}

// Invalidate cache
func (s *UserService) UpdateUser(id uint, updates map[string]interface{}) error {
    if err := s.db.Write.Model(&models.User{}).Where("id = ?", id).
        Updates(updates).Error; err != nil {
        return err
    }

    // Clear cache
    cacheKey := fmt.Sprintf("user:%d", id)
    s.redis.CacheDelete("users", cacheKey)

    return nil
}
```

### Rate Limiting Example

```go
func (h *Handler) RateLimitedEndpoint(c *gin.Context) {
    userID := c.GetUint("user_id")
    key := fmt.Sprintf("api_calls:%d:%s", userID, time.Now().Format("2006-01-02-15"))

    // Allow 100 requests per hour
    allowed, remaining, err := h.redis.RateLimitCheck(key, 100, time.Hour)
    if err != nil {
        // Handle error - maybe allow request
        c.Next()
        return
    }

    c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

    if !allowed {
        utils.ErrorResponse(c, 429, "Rate limit exceeded", "RATE_LIMIT", nil)
        return
    }

    // Process request
    utils.SuccessResponse(c, "Request processed", nil)
}
```

### Session Management

```go
// Store session
sessionData := map[string]interface{}{
    "user_id": user.ID,
    "email":   user.Email,
    "role":    user.Role,
}

sessionID := utils.GenerateUUID()
if err := redis.SessionSet(sessionID, sessionData, 24*time.Hour); err != nil {
    return err
}

// Set cookie
c.SetCookie("session_id", sessionID, 86400, "/", "", true, true)

// Retrieve session
var session map[string]interface{}
if err := redis.SessionGet(sessionID, &session); err != nil {
    // Session not found or expired
}
```

## Encryption/Decryption

### Encrypt Sensitive Data

```go
import "github.com/yourusername/boilerplate-api/utils"

// Encrypt data
sensitiveData := "This is confidential information"
encryptionKey := config.Get().Encryption.Key

encrypted, err := utils.Encrypt(sensitiveData, encryptionKey)
if err != nil {
    log.Fatal("Encryption failed:", err)
}

fmt.Println("Encrypted:", encrypted)

// Decrypt data
decrypted, err := utils.Decrypt(encrypted, encryptionKey)
if err != nil {
    log.Fatal("Decryption failed:", err)
}

fmt.Println("Decrypted:", decrypted)
```

### Password Hashing

```go
// Hash password
password := "SecurePassword123!"
hashedPassword, err := utils.HashPassword(password)
if err != nil {
    return err
}

// Store hashedPassword in database

// Verify password
isValid := utils.CheckPassword(password, hashedPassword)
if !isValid {
    return errors.New("invalid password")
}
```

### Generate Secure Tokens

```go
// Generate secure random token
token, err := utils.GenerateSecureToken(32)
if err != nil {
    return err
}

// Use for password reset, email verification, etc.
resetToken := utils.GeneratePasswordResetToken()
emailToken := utils.GenerateEmailVerificationToken()
```

## Advanced Examples

### Custom Middleware

```go
// Create custom middleware
func CustomMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Before request
        start := time.Now()

        // Add custom headers
        c.Header("X-Request-ID", utils.GenerateUUID())

        // Process request
        c.Next()

        // After request
        latency := time.Since(start)
        logger.WithFields(logrus.Fields{
            "latency": latency,
            "status":  c.Writer.Status(),
            "path":    c.Request.URL.Path,
        }).Info("Request processed")
    }
}

// Use in router
router.Use(CustomMiddleware())
```

### Background Jobs with Redis

```go
// Producer
func QueueEmailJob(userID uint, emailType string) error {
    job := map[string]interface{}{
        "user_id": userID,
        "type":    emailType,
        "queued_at": time.Now(),
    }

    return redis.RPush("email_queue", job)
}

// Consumer (run in separate goroutine/process)
func ProcessEmailQueue() {
    for {
        // Block until job is available
        result, err := redis.BLPop("email_queue", 0)
        if err != nil {
            time.Sleep(5 * time.Second)
            continue
        }

        var job map[string]interface{}
        if err := json.Unmarshal([]byte(result), &job); err != nil {
            continue
        }

        // Process job
        processEmailJob(job)
    }
}
```

### Health Check Implementation

```go
func (h *HealthHandler) DetailedHealthCheck(c *gin.Context) {
    health := map[string]interface{}{
        "status": "healthy",
        "timestamp": time.Now(),
        "services": map[string]string{},
    }

    // Check database
    if err := database.HealthCheck(); err != nil {
        health["status"] = "unhealthy"
        health["services"].(map[string]string)["database"] = err.Error()
    } else {
        health["services"].(map[string]string)["database"] = "healthy"
    }

    // Check Redis
    if err := h.redis.HealthCheck(); err != nil {
        health["status"] = "unhealthy"
        health["services"].(map[string]string)["redis"] = err.Error()
    } else {
        health["services"].(map[string]string)["redis"] = "healthy"
    }

    // Check disk space
    if diskUsage, err := getDiskUsage(); err == nil {
        health["disk_usage"] = diskUsage
    }

    // Check memory usage
    if memUsage, err := getMemoryUsage(); err == nil {
        health["memory_usage"] = memUsage
    }

    statusCode := http.StatusOK
    if health["status"] != "healthy" {
        statusCode = http.StatusServiceUnavailable
    }

    c.JSON(statusCode, health)
}
```

### Chunked File Upload

```javascript
// Client-side chunked upload
async function uploadLargeFile(file) {
  const chunkSize = 5 * 1024 * 1024; // 5MB chunks
  const totalChunks = Math.ceil(file.size / chunkSize);
  const fileId = generateUUID();

  for (let i = 0; i < totalChunks; i++) {
    const start = i * chunkSize;
    const end = Math.min(start + chunkSize, file.size);
    const chunk = file.slice(start, end);

    const formData = new FormData();
    formData.append("chunk", chunk);
    formData.append("fileId", fileId);
    formData.append("chunkNumber", i);
    formData.append("totalChunks", totalChunks);

    const response = await fetch("/api/v1/upload/chunk", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
      },
      body: formData,
    });

    if (!response.ok) {
      throw new Error(`Chunk ${i} upload failed`);
    }

    // Update progress
    const progress = ((i + 1) / totalChunks) * 100;
    updateProgressBar(progress);
  }

  // Finalize upload
  const finalizeResponse = await fetch("/api/v1/upload/finalize", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      fileId: fileId,
      filename: file.name,
      totalChunks: totalChunks,
    }),
  });

  return await finalizeResponse.json();
}
```

### Server-Sent Events (SSE) for Real-time Updates

```go
func (h *Handler) StreamEvents(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    // Create event channel
    eventChan := make(chan string)

    // Subscribe to Redis pub/sub
    pubsub := h.redis.Subscribe("events")
    go func() {
        for msg := range pubsub.Channel() {
            eventChan <- msg.Payload
        }
    }()

    // Send events to client
    c.Stream(func(w io.Writer) bool {
        select {
        case event := <-eventChan:
            c.SSEvent("message", event)
            return true
        case <-c.Request.Context().Done():
            return false
        }
    })
}
```

### GraphQL Integration Example

```go
// Add GraphQL handler (requires additional setup)
func GraphQLHandler() gin.HandlerFunc {
    h := handler.NewDefaultServer(generated.NewExecutableSchema(
        generated.Config{Resolvers: &resolver.Resolver{}},
    ))

    return func(c *gin.Context) {
        h.ServeHTTP(c.Writer, c.Request)
    }
}

// Add to router
router.POST("/graphql", GraphQLHandler())
router.GET("/playground", playgroundHandler())
```

## Testing Examples

### Unit Test Example

```go
func TestUserService_CreateUser(t *testing.T) {
    // Setup
    db, mock, err := sqlmock.New()
    assert.NoError(t, err)
    defer db.Close()

    gormDB, err := gorm.Open(mysql.New(mysql.Config{
        Conn: db,
    }), &gorm.Config{})
    assert.NoError(t, err)

    service := NewUserService(&database.DB{Write: gormDB, Read: gormDB})

    // Test
    user := &models.User{
        Email: "test@example.com",
        Name:  "Test User",
    }

    mock.ExpectBegin()
    mock.ExpectExec("INSERT INTO `users`").
        WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectCommit()

    err = service.CreateUser(user)
    assert.NoError(t, err)
    assert.Equal(t, uint(1), user.ID)
}
```

### Integration Test Example

```go
func TestAuthEndpoint(t *testing.T) {
    // Setup test router
    router := setupTestRouter()

    // Test registration
    body := `{"email":"test@example.com","password":"Test123!","confirm_password":"Test123!","name":"Test User"}`

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    router.ServeHTTP(w, req)

    assert.Equal(t, 201, w.Code)

    var response map[string]interface{}
    err := json.Unmarshal(w.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.True(t, response["success"].(bool))
}
```

## Performance Optimization Tips

1. **Use connection pooling** - Configure database connection pools appropriately
2. **Implement caching** - Cache frequently accessed data in Redis
3. **Use pagination** - Never return unlimited results
4. **Optimize queries** - Use indexes and avoid N+1 queries
5. **Compress responses** - Use gzip middleware for large responses
6. **Rate limiting** - Protect endpoints from abuse
7. **Background jobs** - Process heavy tasks asynchronously
8. **CDN for static files** - Serve uploads/videos through CDN
9. **Database read replicas** - Distribute read load
10. **Monitoring** - Use metrics to identify bottlenecks

## Security Best Practices

1. **Always validate input** - Use struct tags for validation
2. **Sanitize user data** - Prevent XSS and SQL injection
3. **Use HTTPS** - Enable TLS in production
4. **Secure headers** - Use security middleware
5. **Rate limiting** - Prevent brute force attacks
6. **JWT expiration** - Set appropriate token lifetimes
7. **Encrypt sensitive data** - Use provided encryption utilities
8. **Audit logging** - Log security-relevant events
9. **CORS configuration** - Restrict allowed origins
10. **Keep dependencies updated** - Regular security updates# Boilerplate API - Feature Usage Examples

This document provides comprehensive examples of how to use each feature of the Boilerplate API.

## Table of Contents

- [Authentication](#authentication)
- [User Management](#user-management)
- [File Upload](#file-upload)
- [WebSocket](#websocket)
- [Video Streaming](#video-streaming)
- [gRPC](#grpc)
- [Database Operations](#database-operations)
- [Redis Caching](#redis-caching)
- [Encryption/Decryption](#encryptiondecryption)

## Authentication

### Register a New User

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123!",
    "confirm_password": "SecurePass123!",
    "name": "John Doe"
  }'
```

Response:

```json
{
  "success": true,
  "message": "Registration successful",
  "data": {
    "user": {
      "id": 1,
      "email": "user@example.com",
      "name": "John Doe",
      "role": "user",
      "is_active": true,
      "email_verified": false,
      "created_at": "2024-01-20T10:00:00Z"
    },
    "tokens": {
      "access_token": "eyJhbGciOiJIUzI1NiIs...",
      "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
      "token_type": "Bearer",
      "expires_in": 86400
    }
  }
}
```

### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123!"
  }'
```

### Refresh Token

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
  }'
```

### Using Authentication in Requests

```bash
# Set token variable
TOKEN="eyJhbGciOiJIUzI1NiIs..."

# Use in subsequent requests
curl -X GET http://localhost:8080/api/v1/users/profile \
  -H "Authorization: Bearer $TOKEN"
```

## User Management

### Get User Profile

```bash
curl -X GET http://localhost:8080/api/v1/users/profile \
  -H "Authorization: Bearer $TOKEN"
```

### Update Profile

```bash
curl -X PUT http://localhost:8080/api/v1/users/profile \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Smith",
    "avatar": "https://example.com/avatar.jpg"
  }'
```

### Admin: List Users with Pagination

```bash
curl -X GET "http://localhost:8080/api/v1/admin/users?page=1&per_page=20&sort_by=created_at&sort_order=desc" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## File Upload

### Upload Single File

```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/document.pdf"
```

Response:

```json
{
  "success": true,
  "message": "File uploaded successfully",
  "data": {
    "filename": "1705749600_a1b2c3d4.pdf",
    "original_name": "document.pdf",
    "size": 1048576,
    "mime_type": "application/pdf",
    "extension": ".pdf",
    "url": "/uploads/2024/01/20/1705749600_a1b2c3d4.pdf",
    "hash": "d41d8cd98f00b204e9800998ecf8427e",
    "uploaded_at": "2024-01-20T10:00:00Z"
  }
}
```

### Upload Multiple Files

```bash
curl -X POST http://localhost:8080/api/v1/upload/multiple \
  -H "Authorization: Bearer $TOKEN" \
  -F "files=@/path/to/image1.jpg" \
  -F "files=@/path/to/image2.jpg" \
  -F "files=@/path/to/document.pdf"
```

### Upload User Avatar

```bash
curl -X POST http://localhost:8080/api/v1/users/avatar \
  -H "Authorization: Bearer $TOKEN" \
  -F "avatar=@/path/to/avatar.jpg"
```

## WebSocket

### JavaScript WebSocket Client

```javascript
// Connect to WebSocket
const ws = new WebSocket("ws://localhost:8080/ws");

// Connection opened
ws.onopen = function (event) {
  console.log("Connected to WebSocket");

  // Send authentication if needed
  ws.send(
    JSON.stringify({
      type: "auth",
      data: { token: "your-jwt-token" },
    })
  );
};

// Listen for messages
ws.onmessage = function (event) {
  const message = JSON.parse(event.data);
  console.log("Received:", message);

  switch (message.type) {
    case "welcome":
      console.log("Welcome message:", message.data);
      break;
    case "notification":
      showNotification(message.data);
      break;
    case "chat_message":
      displayChatMessage(message.data);
      break;
  }
};

// Send messages
function sendMessage(type, data) {
  ws.send(
    JSON.stringify({
      type: type,
      data: data,
    })
  );
}

// Join a room
sendMessage("join_room", { room: "chat_room_123" });

// Send a chat message
sendMessage("broadcast", {
  room: "chat_room_123",
  message: "Hello everyone!",
});

// Handle errors
ws.onerror = function (error) {
  console.error("WebSocket error:", error);
};

// Handle connection close
ws.onclose = function (event) {
  console.log("WebSocket connection closed");
};
```

### Go WebSocket Client

```go
package main

import (
    "log"
    "github.com/gorilla/websocket"
)

func main() {
    // Connect to WebSocket
    conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
    if err != nil {
        log.Fatal("dial:", err)
    }
    defer conn.Close()

    // Send message
    msg := map[string]interface{}{
        "type": "ping",
        "data": map[string]string{"message": "Hello"},
    }

    if err := conn.WriteJSON(msg); err != nil {
        log.Println("write:", err)
        return
    }

    // Read messages
    for {
        var message map[string]interface{}
        err := conn.ReadJSON(&message)
        if err != nil {
            log.Println("read:", err)
            break
        }
        log.Printf("recv: %v", message)
    }
}
```

## Video Streaming

### Stream Video with Range Support

```bash
# Request full video
curl -X GET http://localhost:8080/api/v1/stream/video/video123 \
  -H "Authorization: Bearer $TOKEN" \
  -o video.mp4

# Request specific range (for video players)
curl -X GET http://localhost:8080/api/v1/stream/video/video123 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Range: bytes=0-1048575" \
  -o video_part.mp4
```

### HTML5 Video Player Example

```html
<!DOCTYPE html>
<html>
  <head>
    <title>Video Streaming</title>
  </head>
  <body>
    <video id="videoPlayer" width="640" height="480" controls>
      <source src="/api/v1/stream/video/video123" type="video/mp4" />
      Your browser does not support the video tag.
    </video>

    <script>
      // Add authorization header for protected videos
      const video = document.getElementById("videoPlayer");
      const token = localStorage.getItem("auth_token");

      // For protected content, you might need to use fetch API
      fetch("/api/v1/stream/video/video123", {
        headers: {
          Authorization: `Bearer ${token}`,
          Range: "bytes=0-",
        },
      })
        .then((response) => response.blob())
        .then((blob) => {
          video.src = URL.createObjectURL(blob);
        });
    </script>
  </body>
</html>
```

### HLS Streaming

```bash
# Get HLS playlist
curl -X GET http://localhost:8080/api/v1/stream/hls/video123/playlist.m3u8 \
  -H "Authorization: Bearer $TOKEN"

# Get HLS segment
curl -X GET http://localhost:8080/api/v1/stream/hls/video123/segment0.ts \
  -H "Authorization: Bearer $TOKEN"
```

## gRPC

### gRPC Go Client Example

```go
package main

import (
    "context"
    "log"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    pb "github.com/yourusername/boilerplate-api/grpc/proto"
)

func main() {
    // Connect to gRPC server
    conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    // Create clients
    authClient := pb.NewAuthServiceClient(conn)
    userClient := pb.NewUserServiceClient(conn)

    ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
    defer cancel()

    // Login
    loginResp, err := authClient.Login(ctx, &pb.LoginRequest{
        Email:    "user@example.com",
        Password: "SecurePass123!",
    })
    if err != nil {
        log.Fatalf("Login failed: %v", err)
    }

    log.Printf("Logged in successfully. Token: %s", loginResp.AccessToken)

    // Get user
    userResp, err := userClient.GetUser(ctx, &pb.GetUserRequest{
        Id: loginResp.User.Id,
    })
    if err != nil {
        log.Fatalf("Failed to get user: %v", err)
    }

    log.Printf("User: %+v", userResp)

    // List users with pagination
    listResp, err := userClient.ListUsers(ctx, &pb.ListUsersRequest{
        Page:    1,
        PerPage: 10,
        Filter: &pb.UserFilter{
            Search: "john",
            Role:   "user",
        },
    })
    if err != nil {
        log.Fatalf("Failed to list users: %v", err)
    }

    log.Printf("Found %d users", len(listResp.Users))

    // Stream users
    stream, err := userClient.StreamUsers(ctx, &pb.StreamUsersRequest{
        Filter: &pb.UserFilter{
            IsActive: true,
        },
    })
    if err != nil {
        log.Fatalf("Failed to stream users: %v", err)
    }

    for {
        user, err := stream.Recv()
        if err != nil {
            break
        }
        log.Printf("Streamed user: %s", user.Email)
    }
}
```

### gRPC Python Client Example

```python
import grpc
import user_pb2
import user_pb2_grpc
import auth_pb2
import auth_pb2_grpc

# Connect to gRPC server
channel = grpc.insecure_channel('localhost:50051')
auth_stub = auth_pb2_grpc.AuthServiceStub(channel)
user_stub = user_pb2_grpc.UserServiceStub(channel)

# Login
login_request = auth_pb2.LoginRequest(
    email="user@example.com",
    password="SecurePass123!"
)
login_response = auth_stub.Login(login_request)
print(f"Access token: {login_response.access_token}")

# Get user
user_request = user_pb2.GetUserRequest(id=1)
user_response = user_stub.GetUser(user_request)
print(f
```
