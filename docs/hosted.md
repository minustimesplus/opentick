# Running the hosted instance

The hosted instance is the same server as a self-hosted one, configured with
environment variables:

- **Heroku** runs the Docker image on one Basic dyno.
- **Neon** provides PostgreSQL.
- **S3 or Cloudflare R2** stores attachments and backgrounds, because a dyno's disk is wiped on every restart.
- **Clerk** handles sign up and sign in (email and password, Google, Apple).

The `Deploy hosted` workflow builds the image and releases it to Heroku on
every push to `main`.

## How sign in works

The login screen runs Clerk's sign in flows in the browser. Once Clerk has a
session, the page sends its session token to `POST /api/v2/auth/clerk`. The
server checks the token against Clerk's signing keys, loads the user's email
and name from Clerk's backend API, creates the user on first sign in, and
returns a regular session. From then on the app works exactly as when self
hosted.

Users are matched to Clerk by their Clerk user id, so changing an email in
Clerk updates it here on the next sign in.

## Environment variables

| Variable | Value |
|---|---|
| `VIKUNJA_SERVICE_PUBLICURL` | `https://<app>.herokuapp.com/` or your domain |
| `VIKUNJA_SERVICE_SECRET` | A long random string. Without it every restart signs everyone out. |
| `VIKUNJA_DATABASE_TYPE` | `postgres` |
| `VIKUNJA_DATABASE_HOST` | Neon host, e.g. `ep-cool-name-123456.us-east-1.aws.neon.tech` |
| `VIKUNJA_DATABASE_USER` | Neon role |
| `VIKUNJA_DATABASE_PASSWORD` | Neon password |
| `VIKUNJA_DATABASE_DATABASE` | `neondb` |
| `VIKUNJA_DATABASE_SSLMODE` | `require` |
| `VIKUNJA_FILES_TYPE` | `s3` |
| `VIKUNJA_FILES_S3_ENDPOINT` | `https://s3.us-east-1.amazonaws.com` or `https://<account>.r2.cloudflarestorage.com` |
| `VIKUNJA_FILES_S3_BUCKET` | Bucket name |
| `VIKUNJA_FILES_S3_REGION` | `us-east-1` (R2: `auto`) |
| `VIKUNJA_FILES_S3_ACCESSKEY` | Access key id |
| `VIKUNJA_FILES_S3_SECRETKEY` | Secret access key |
| `VIKUNJA_AUTH_LOCAL_ENABLED` | `false` |
| `VIKUNJA_AUTH_CLERK_ENABLED` | `true` |
| `VIKUNJA_AUTH_CLERK_PUBLISHABLEKEY` | `pk_live_...` |
| `VIKUNJA_AUTH_CLERK_SECRETKEY` | `sk_live_...` |
| `VIKUNJA_AUTH_CLERK_SOCIALPROVIDERS` | `google apple` (space separated, in display order) |

Heroku sets `PORT` and the server listens on it automatically.

Use Neon's direct connection string, not the `-pooler` one.

## Clerk settings

- **Email**: allow sign up with email, require email verification by code, allow password sign in.
- **Username, phone, first and last name**: not required. The login screen does not ask for them.
- **Social connections**: enable the ones listed in `VIKUNJA_AUTH_CLERK_SOCIALPROVIDERS`. In production each needs its own OAuth credentials.
- **Domains**: a production instance needs your own domain, with the DNS records Clerk lists.

## Desktop app

Clerk production instances only accept requests from their own domain. The
desktop app serves the UI from `127.0.0.1`, so it shows a notice instead of
the sign in form for a hosted server. Clerk development keys (`pk_test_...`)
work from the desktop app.
