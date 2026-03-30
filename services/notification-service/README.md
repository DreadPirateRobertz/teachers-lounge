# notification-service

Notification service for Teachers Lounge — handles push notifications (FCM),
email (SendGrid), in-app notification feed, and notification preferences.

## API Endpoints

All endpoints except `/health` require a valid JWT Bearer token.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/notifications/send` | Send notification to user via specified channels |
| GET | `/notifications` | List in-app notifications (paginated) |
| PATCH | `/notifications/{id}/read` | Mark single notification as read |
| POST | `/notifications/read-all` | Mark all notifications as read |
| GET | `/notifications/preferences` | Get notification preferences |
| PUT | `/notifications/preferences` | Update notification preferences |
| POST | `/notifications/devices` | Register FCM device token |
| DELETE | `/notifications/devices/{token}` | Unregister device token |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | — | Postgres connection string |
| `JWT_SECRET` | Yes | — | JWT signing secret |
| `PORT` | No | `8084` | HTTP listen port |
| `REDIS_ADDR` | No | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | No | — | Redis password |
| `FCM_SERVER_KEY` | No | — | Firebase Cloud Messaging server key |
| `FCM_PROJECT_ID` | No | — | Firebase project ID |
| `SENDGRID_API_KEY` | No | — | SendGrid API key |
| `SENDGRID_FROM_EMAIL` | No | `noreply@teacherslounge.app` | Sender email |
| `SENDGRID_FROM_NAME` | No | `Teachers Lounge` | Sender display name |

FCM and SendGrid are optional — if keys are not set, those channels report errors gracefully.

See root README and [docs/tv-tutor-design.md](../../docs/tv-tutor-design.md) for spec.
