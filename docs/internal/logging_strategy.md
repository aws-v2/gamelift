# Logging Strategy & Conventions

Consistent, structured logging is the backbone of our observability. We use `go.uber.org/zap` for high-performance, structured logging.

## 1. Core Principles

- **Structured**: Use key-value pairs. No formatted strings in message bodies.
- **Traceable**: Every log should belong to an identifiable flow (Request ID, Session ID, Game ID).
- **Consistently Suffixes**:
    - `_RECEIVED`: Action initiated (e.g., HTTP request or NATS message).
    - `_SUCCESS`: Operation completed successfully.
    - `_FAILED`: Terminal failure.
    - `_NOT_FOUND`: Resource missing (usually `Warnw`).

## 2. Event Naming Convention

Always use **UPPERCASE_WITH_UNDERSCORES**.

| Prefix | Layer | Example |
| :--- | :--- | :--- |
| `HANDLER_` | Transport (HTTP/WS) | `HANDLER_INIT_UPLOAD_SUCCESS` |
| `SESSION_` | Business Logic | `SESSION_CREATE_PROVISIONING` |
| `GAME_` | Repository / Logic | `GAME_GET_FOUND` |
| `MINIO_` | Infrastructure (Storage) | `MINIO_DOWNLOAD_FILE_SUCCESS` |
| `DB_` | Infrastructure (Database) | `DB_MIGRATE_STARTING` |
| `WEBRTC_` | Real-time Signaling | `WEBRTC_SIGNALING_MESSAGE_RELAYED` |

## 3. Real-World Examples

### API Entry Point (Transport)
In `internal/transport/http/handlers/game_handler.go`, we attach a `request_id` to the context logger:
```go
reqID := uuid.New().String()
log := h.log.With("layer", "handler", "method", "InitUpload", "request_id", reqID)
log.Infow("HANDLER_INIT_UPLOAD_RECEIVED")
```

### Event Listener (Messaging)
In `internal/transport/nats/s3_listener.go`, we log the arrival of events:
```go
l.logger.Debugw("S3_STORED_MESSAGE_RECEIVED", "bytes", len(msg.Data))
// ...
l.logger.Infow("S3_STORED_UPLOAD_SUCCESS", "game_id", payload.GameID, "arn", payload.StorageARN)
```

### Business Logic (Service)
In `internal/application/session_service.go`, we trace the provisioning handover:
```go
s.logger.Infow("SESSION_CREATE_PROVISIONING", "session_id", session.ID, "game_id", session.GameID)
if err := s.provisioningSvc.ProvisionGame(session.GameID, domain.StreamingModeState, session.ID); err != nil {
    s.logger.Errorw("SESSION_CREATE_PROVISION_FAILED", "session_id", session.ID, "error", err)
}
```

## 4. Standard Keys

| Key | Format | Example |
| :--- | :--- | :--- |
| `user_id` | UUID String | `user_123` |
| `game_id` | UUID String | `game_456` |
| `session_id` | UUID String | `sess_789` |
| `error` | error | `err` |
| `remote_addr` | IP:Port | `192.168.1.1:54321` |
| `duration_ms` | int64 | `150` |

## 5. Log Levels

- **DEBUG**: High-volume data (e.g., packet relay, NATS message bodies).
- **INFO**: Meaningful lifecycle events and transitions.
- **WARN**: Unexpected but handled scenarios (e.g., user error, 404).
- **ERROR**: Infrastructure failures, bugs, or unrecoverable issues.
