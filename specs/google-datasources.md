# Google Calendar and Gmail DataSources

> Spec for adding Google Calendar and Gmail as datasources to Kora.

## Overview

Two datasource types sharing Google OAuth:
- **Google Calendar** (`google_calendar`) - Meetings and calendar events
- **Gmail** (`gmail`) - Unread/unreplied emails from real people

Users can have multiple Google accounts. Each account spawns both datasource types.

## Authentication

### OAuth Flow

1. Kora loads a Google datasource that isn't authenticated
2. Opens browser to Google OAuth consent screen
3. User authorizes (calendar.readonly + gmail.readonly scopes)
4. Redirect to localhost callback with auth code
5. Kora exchanges for access + refresh tokens
6. Stored in macOS Keychain: `google-oauth-{email}`

Auth is implicit—triggered when needed. Kora bundles its own OAuth client credentials. Token refresh is automatic.

See `specs/google-oauth-flow.md` for detailed OAuth mechanics.

### Multi-Account

Each email address has independent credentials stored in Keychain.

```
work@company.com   → GoogleCalendarDataSource + GmailDataSource (shared cred)
personal@gmail.com → GoogleCalendarDataSource + GmailDataSource (shared cred)
```

## Google Calendar DataSource

**Source**: `google_calendar`

**Fetches** events within `FetchOptions.Since` window:
- User's accepted/tentative meetings
- Events needing RSVP
- Events user is organizing with pending RSVPs
- All-day events for context

Declined events excluded. Recurring events: next occurrence only.

### Event Types

| Type | When | Priority | Action |
|------|------|----------|--------|
| `calendar_upcoming` | Starts within 1hr | High | no |
| `calendar_needs_rsvp` | No response yet | Medium | yes |
| `calendar_organizer_pending` | Organizing, awaiting RSVPs | Medium | yes |
| `calendar_tentative` | Marked tentative | Medium | no |
| `calendar_meeting` | Accepted | Medium | no |
| `calendar_all_day` | All-day event | Info | no |

### Metadata

`calendar_id`, `event_id`, `start_time`, `end_time`, `is_all_day`, `organizer_email`, `organizer_name`, `attendees`, `attendee_count`, `response_status`, `pending_rsvps`, `location`, `conference_url`, `description`

## Gmail DataSource

**Source**: `gmail`

**Fetches** within `FetchOptions.Since`:
- All unread emails from real people (not automated/lists)
- All unreplied emails where user is direct recipient

Excludes: mailing lists (List-Unsubscribe header), auto-generated emails, read+replied emails.

### Event Types

| Type | When | Priority | Action |
|------|------|----------|--------|
| `email_important` | Important sender or Gmail-marked important | High | yes |
| `email_direct` | User in To: field, unread | Medium | yes |
| `email_cc` | User in CC: field | Low | no |

### Metadata

`message_id`, `thread_id`, `from_email`, `from_name`, `to_addresses`, `cc_addresses`, `subject`, `snippet`, `labels`, `is_unread`, `is_important`, `is_starred`, `has_attachments`, `received_at`

## Configuration

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

Calendars and email accounts are configured independently. Each entry references a Google account by email address.

## Security

Per EFA 0002:
- Keychain storage only
- Tokens never logged
- Read-only scopes
- TLS 1.2+

## EFA Updates Required

**EFA 0001**: Add sources (`google_calendar`, `gmail`), event types, metadata keys

**EFA 0002**: Add `google` service, `google-oauth-*` keychain keys
