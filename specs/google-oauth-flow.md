# Google OAuth Flow

> How Kora authenticates with Google Calendar and Gmail APIs.

## Model

Users configure 1-N calendars and 1-N email accounts. Auth is implicit—when Kora loads a Google datasource that isn't authenticated, it triggers the OAuth flow.

```yaml
google:
  calendars:
    - email: work@company.com
      calendar_id: primary
    - email: personal@gmail.com
      calendar_id: primary
  gmail:
    - email: work@company.com
      important_senders: [manager@company.com]
    - email: personal@gmail.com
```

First run for each email triggers browser OAuth. Subsequent runs use stored tokens.

## OAuth Basics

Google uses OAuth 2.0 Authorization Code flow.

| Concept | What it is |
|---------|------------|
| OAuth Client | Kora's registered app (bundled client ID + secret) |
| Scopes | `calendar.readonly`, `gmail.readonly` |
| Access Token | Short-lived (~1hr), used for API calls |
| Refresh Token | Long-lived, used to get new access tokens |

## Auth Flow (First Time Per Email)

When Kora encounters an unauthenticated Google email:

```
Kora                                              Google
 │                                                   │
 │  No token for work@company.com                    │
 │  Start localhost callback server                  │
 │  Open browser ──────────────────────────────────> │
 │       accounts.google.com/o/oauth2/auth           │
 │       ?scope=calendar.readonly+gmail.readonly     │
 │       &redirect_uri=localhost:PORT/callback       │
 │                                                   │
 │                    User sees consent screen       │
 │                    "Kora wants to view            │
 │                     calendars and email"          │
 │                    User clicks Allow              │
 │                                                   │
 │  <─────────────────────────────────────────────── │
 │  Redirect to localhost?code=AUTH_CODE             │
 │                                                   │
 │  Exchange code for tokens ──────────────────────> │
 │  <─────────────────────────────────────────────── │
 │  {access_token, refresh_token, expires_in}        │
 │                                                   │
 │  Store in Keychain: google-oauth-{email}          │
 │  Continue with API calls                          │
```

## Token Refresh (Automatic)

Before each API call, Kora checks token expiry. If expired (or within 5min), refreshes silently:

```
Kora                                              Google
 │                                                   │
 │  Token expires in 3 minutes                       │
 │  POST /token {refresh_token} ───────────────────> │
 │  <─────────────────────────────────────────────── │
 │  {new access_token, expires_in: 3600}             │
 │  Update Keychain                                  │
 │  Continue with API call                           │
```

## Storage

Keychain key per email: `google-oauth-{email}`

```json
{
  "access_token": "ya29.a0AfH6SM...",
  "refresh_token": "1//0eXx...",
  "expiry": "2025-01-15T10:00:00Z",
  "email": "work@company.com"
}
```

## Revoking Access

User revokes at https://myaccount.google.com/permissions → Remove "Kora"

Next Kora run re-triggers OAuth flow for that email.

## Security

- Tokens in macOS Keychain only
- Never logged (hash fingerprint for correlation)
- Read-only scopes
- Refresh over HTTPS
- Per EFA 0002 rules
