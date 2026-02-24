# Design: User Management

**Status:** Proposed
**Author:** —
**Date:** 2026-02-24

---

## 1. Problem Statement

The backend has zero user management endpoints. Admins cannot list users, change roles, deactivate accounts, or reset passwords without direct database access. Users cannot change or reset their own passwords. This blocks basic operational needs for any production deployment.

## 2. Goals

- Admin user management: list, create, update role/active status, reset password.
- Authenticated password change (PUT /me/password).
- Self-service password reset via email when SMTP is configured.
- Graceful fallback when email is not configured (frontend shows "contact admin").
- Admin-created accounts with forced password change on first login.
- SSO-compatible design: no premature abstractions, but no blocking decisions.

## 3. Non-Goals

- **Email change** — deferred. Requires re-verification flow and default notification channel update.
- **User deletion** — only deactivation. Hard delete would orphan or cascade event history references.
- **SSO implementation** — future work. Current design accommodates it (see section 7.7).
- **Middleware-level blocking for `must_change_password`** — frontend enforces the redirect. Backend does not restrict API access based on this flag.

## 4. Database Changes

Migration `000021_user_management.up.sql`:

```sql
ALTER TABLE users
    ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_password_reset_tokens_token ON password_reset_tokens(token);
CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
```

Existing users get `is_active = true`, `must_change_password = false` by default — no data migration needed.

## 5. New API Endpoints

### Public (no auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/forgot-password` | Request password reset email. Always returns 200 (prevents email enumeration). Returns 400 only if email is not configured. |
| POST | `/api/v1/auth/reset-password` | Set new password using reset token. |

### Authenticated

| Method | Path | Description |
|--------|------|-------------|
| PUT | `/api/v1/me/password` | Change own password. Requires `current_password` + `new_password`. Clears `must_change_password`. Invalidates all refresh tokens. Returns 204. |
| PATCH | `/api/v1/me` | Update profile (`first_name`, `last_name` only). Returns 200 with updated user. |

### Admin only

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/users` | List users. Pagination (`limit`/`offset`), filter by `role`. Returns total count in response. |
| GET | `/api/v1/users/{id}` | Get user details. |
| POST | `/api/v1/users` | Create user with specified role. Always sets `must_change_password=true`. Creates default email channel. |
| PATCH | `/api/v1/users/{id}` | Update role, `is_active`, `first_name`, `last_name`. Cannot modify self (409). On deactivation: invalidates all refresh tokens. |
| POST | `/api/v1/users/{id}/reset-password` | Set new password for user. Sets `must_change_password=true`. Invalidates refresh tokens. Cannot reset self (409). |

## 6. Modified Behavior

- **Login** (`POST /auth/login`): Check `is_active`. If false, return 403 with message "account deactivated". Include `must_change_password` in the response body (part of User JSON).
- **Token Refresh** (`POST /auth/refresh`): Check `is_active`. If false, return 403 and delete the refresh token.
- **GET /me**: Response now includes `is_active` and `must_change_password` fields.

## 7. Key Design Decisions

### 7.1 EmailSender interface in identity package

**Problem:** The `notifications` module depends on `identity` (via `UserCreatedHandler` for default channel creation), and now `identity` needs to send password reset emails. Importing `notifications` from `identity` creates a circular dependency.

**Solution:** Define a minimal `EmailSender` interface in the `identity` package:

```go
type EmailSender interface {
    SendEmail(ctx context.Context, to, subject, body string) error
}
```

An adapter in `app.go` wraps `email.Sender` from the notifications module. Neither module imports the other directly.

### 7.2 Direct email send (not queued) for password reset

Password reset emails must arrive immediately. The notification queue has poll intervals and retry delays that are acceptable for event notifications but not for password resets. The reset flow bypasses the queue and calls `email.Sender.Send()` directly.

### 7.3 No user deletion, only deactivation

Events reference users via `created_by`. Hard deletion would orphan these references or require `CASCADE SET NULL`, losing audit history. Deactivation (`is_active = false`) is reversible and preserves data integrity.

### 7.4 Self-modification prohibition protects last admin

Two rules enforce this:
1. An admin cannot change their own role.
2. An admin cannot deactivate themselves.

Combined, these guarantee at least one active admin always exists. No explicit "last admin" count check is needed.

### 7.5 must_change_password -- frontend enforcement only

No middleware blocking. The flag is returned in the login response and `GET /me`. The frontend redirects to a password change page. The backend does not restrict API access based on this flag. This is sufficient for a status page service and can be tightened later if needed.

### 7.6 Forgot password -- always 200

To prevent email enumeration, `POST /auth/forgot-password` returns 200 even if the email does not exist in the system. Rate limiting: skip sending if an unexpired token for the same user was created less than 5 minutes ago. Token specification: 32 bytes from `crypto/rand`, hex-encoded (64 characters), 1-hour TTL, single-use (deleted after successful reset).

### 7.7 SSO compatibility

No SSO-specific code is introduced. The current design is compatible because:

- User management (roles, deactivation) is auth-method-agnostic.
- JWT is the common auth token regardless of how the user logged in.
- `password_hash` can be made nullable when SSO users are added.
- `is_active` works universally -- blocks both local and SSO login.
- Email uniqueness is preserved for future account linking.

## 8. Implementation Plan

Four sequential PRs. PR2, PR3, and PR4 all depend on PR1.

Execution order: PR1 -> PR2 -> PR3 -> PR4.

### PR1: Foundation (migration + domain + repository)

- Migration 000021.
- Domain types: add `IsActive`, `MustChangePassword` to `User`; add `PasswordResetToken` entity.
- Config: add `APP_FRONTEND_URL` env var.
- Repository interface: 8 new methods + `UserFilter` struct.
- Postgres implementation: update existing queries to include new columns, implement new methods.
- Unit test mock updates.

### PR2: Auth changes + password change + profile update

- Login and refresh: `is_active` check.
- `PUT /me/password` handler and service logic.
- `PATCH /me` handler and service logic.
- OpenAPI updates for these endpoints.
- Integration tests.

### PR3: Forgot/reset password

- `EmailSender` interface in identity package + adapter wiring in `app.go`.
- `ForgotPassword` and `ResetPassword` service methods.
- Handlers for `/auth/forgot-password` and `/auth/reset-password`.
- OpenAPI updates.
- Integration tests.

### PR4: Admin user management

- `RegisterAdminRoutes`: list, create, get, update, reset-password.
- Service methods: `ListUsers`, `CreateUser`, `UpdateUser`, `AdminResetPassword`.
- OpenAPI updates.
- Integration tests.
- CLAUDE.md update (final, after all PRs merged).

## 9. Testing Strategy

Each PR includes its own tests following the project test matrix:

- **Unit tests** for service logic (mocked repository, mocked email sender).
- **Integration tests** with real Postgres via testcontainers.
- **OpenAPI response validation** against the spec for all new endpoints.
- All existing tests must continue passing.

Key scenarios per PR:

| PR | Test coverage |
|----|---------------|
| PR1 | Repository methods: CRUD, filters, pagination, token lifecycle |
| PR2 | Login blocked when inactive (403), password change happy path, wrong current password (401), token invalidation, profile update validation |
| PR3 | Reset email sent, token expiry, token reuse rejected, rate limiting (skip if recent token exists), nonexistent email (still 200), email disabled (400) |
| PR4 | List with filters/pagination, create user + default channel, role update, deactivation + token invalidation, self-modification blocked (409), RBAC (non-admin gets 403) |

## 10. Configuration

New environment variable:

| Variable | Description | Example |
|----------|-------------|---------|
| `APP_FRONTEND_URL` | Base URL of the frontend application. Used to construct password reset links. | `https://status.example.com` |

Reset link format: `{APP_FRONTEND_URL}/reset-password?token={token}`

The frontend determines email availability via the existing `GET /api/v1/notifications/config` endpoint. If `email` is not in the `available_channels` array, the frontend shows a "contact your administrator" message instead of the forgot-password form. No new backend endpoint is needed for this.
