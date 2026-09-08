# Community V1 Data Catalog

## MySQL

The first five table names retain the repository's legacy GORM naming. New community tables use explicit names.

| Table | Purpose | Important key/index |
|---|---|---|
| `user_of_dbs` | user identity/profile | unique email, unique phone |
| `social_account_of_dbs` | OAuth account binding | provider/open_id/deletion unique; user/provider/deletion unique |
| `user_sign_in_stat_of_dbs` | aggregate sign-in streak | unique user_id |
| `user_sign_in_record_of_dbs` | immutable daily sign-in | unique `(user_id,biz_day)` |
| `user_point_record_of_dbs` | point ledger | unique `(user_id,biz_type,biz_id)` |
| `event_outbox` | pending/published reliable events | `(status,id)` scan index; attempts/last_error |
| `user_relations` | follower → followee relation | unique `(follower_id,followee_id)`; stable list indexes |
| `notes` | note truth and soft-delete status | `(author_id,status,created_at,id)`; `(status,created_at,id)` |
| `note_images` | ordered note image URLs | unique `(note_id,sort_order)` |
| `note_likes` | like relation | unique `(user_id,note_id)` |
| `comments` | published comments | `(note_id,status,created_at,id)` |
| `notifications` | community notification truth/read state | unique `event_id`; `(receiver_id,created_at,id)` |

Schema is currently managed by GORM `AutoMigrate` on DB-using process startup. This is adequate for the local learning project but is not a versioned production migration system.

## Redis DB 1

| Key pattern | Type | Owner | Lifecycle / purpose |
|---|---|---|---|
| `user:ssid:{ssid}` | String | JWT handler | logout marker, access-token TTL |
| `user:refresh:ssid:{ssid}` | String | JWT handler | current refresh JTI, refresh TTL |
| `phone_code:{biz}:{phone}` | String | SMS login | verification code TTL |
| `user:info:{user_id}` | String/JSON | user cache | profile read cache |
| `sign:{user_id}:{yyyy}:{mm}` | Bitmap | sign-in cache | signed calendar days |
| `rank:active:daily:{yyyyMMdd}` | ZSet | activity worker/rank | daily activity ranking |
| `rank:active:monthly:{yyyyMM}` | ZSet | activity worker/rank | monthly activity ranking |
| `activity:log:user:{user_id}` | List | activity worker | latest 100 activity entries |
| `welcome:message:user:{user_id}` | String/JSON | notification-service | welcome message, 90-day TTL |
| `consumer:event:processing:{namespace}:{event_id}` | String | consumers | in-flight marker |
| `consumer:event:done:{namespace}:{event_id}` | String | consumers | completed event marker |
| `worker:event:done:{event_id}` | String | activity worker | atomic activity event dedup |
| `feed:inbox:{user_id}` | ZSet | Feed worker/API | note ID inbox, configured max 500 |
| `note:detail:{note_id}` | String/JSON | Note repository | positive cache or negative tombstone; TTL jitter |
| `hot:note:{yyyyMMddHHmm}` | ZSet | Hot worker | minute score bucket |
| `hot:event:done:{event_id}` | String | Hot worker | atomic event dedup |
| `hot:note:snapshot:{unix_minute}` | ZSet | Hot API | materialized stable page snapshot |
| `hot:note:snapshot:ready:{unix_minute}` | String | Hot API | snapshot existence marker |

`rank:version:*` and generic `idempotent:*` code exists but is not connected to the current runtime graph; it must not be described as an active consistency mechanism.

## Elasticsearch

| Index | Truth status | Document ID | Fields | Rebuild |
|---|---|---|---|---|
| `community_notes` | derived | note ID | note_id, author_id, title, content, created_at, status | `cmd/search-reindex` from MySQL |
