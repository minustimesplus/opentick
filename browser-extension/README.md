# OpenTick LinkedIn enrichment (Chrome extension)

Fills your OpenTick contacts' work history and education from LinkedIn. It reads
profiles through **your own logged-in LinkedIn session**, the way the Personal CRM
extension did, and sends them straight to your OpenTick server's `/api/v2`.

- **Save to OpenTick**: on any LinkedIn profile, click the extension's toolbar
  button (or the blue button on the page) to save that person. With *Add new
  people* on, someone who isn't a contact yet is added.
- **Fill missing**: works through the contacts that have a LinkedIn URL but no
  experience or education yet, one at a time in a background tab: 45–90 s
  apart and at most 80 a day by default. It stops on a LinkedIn rate limit,
  security check/captcha or logged-out session, says so, and never tries to
  get around them.
- **Auto-save while browsing** (optional, off by default): opening the profile
  of an existing contact saves it, at most once a day per profile.
- **Progress page**: running state, queue size, done today, next run, the
  sync outbox and a log with links to the contacts in OpenTick.

Everything that is read goes into an outbox in the extension's storage first
and is pushed to the server from there, with retries and backoff. Nothing read
is lost while the server is unreachable; pending and rejected pushes are on the
progress page, where you can retry or discard them.

## Install (unpacked)

1. Open `chrome://extensions` and turn on **Developer mode**.
2. Click **Load unpacked** and pick this `browser-extension/` folder.
3. The options page opens. Keep it open for the next step.

It needs Chrome 120 or newer (or another Chromium browser).

## Connect it to OpenTick

1. In OpenTick, go to **People → Import → Browser extension** and click
   **Create token**. The token can only call the four LinkedIn routes below and
   nothing else; it expires after a year and can be deleted under
   Settings. Copy it right away: it is shown only once.
   (To make one by hand instead, create an API token with the `contacts`
   permissions `linkedin_profiles`, `linkedin_failures`, `linkedin_queue` and
   `linkedin_lookup`.)
2. In the extension's options, paste the server address (e.g.
   `https://tick.example.com`) and the token, and click **Save & connect**.
   Chrome asks to let the extension access that server. That is the only site
   besides linkedin.com it can reach.

## Settings

| Setting | Default | |
|---|---|---|
| Seconds between profiles | 45–90 | Random pause between two profiles. 30 s is the minimum. |
| Profiles per day | 80 | Counts Fill missing and auto-saves; resets at local midnight. |
| Active hours | off | Only read profiles between two times (may span midnight). |
| Start when the browser starts | off | Starts Fill missing on browser startup. |
| Start every day at | off | Starts Fill missing daily at a time. |
| Show a Save button on profiles | on | The floating button on LinkedIn profile pages. |
| Add new people | on | Manual saves add people who aren't contacts yet. |
| Auto-save existing contacts | off | Saves profiles of contacts as you open them. |

A running Fill missing keeps going by itself: outside active hours or after
the daily limit it waits and continues later. Automatic starts don't resume
after a security check (solve it and click Resume) and wait a day after a rate
limit.

## Server routes

All under `/api/v2`, authenticated with the token:

| Route | Token permission |
|---|---|
| `POST /contacts/linkedin/profiles` save a profile (`create` adds new people) | `contacts.linkedin_profiles` |
| `POST /contacts/linkedin/failures` record an unreadable profile | `contacts.linkedin_failures` |
| `GET /contacts/linkedin/queue` contacts still missing history | `contacts.linkedin_queue` |
| `GET /contacts/linkedin/lookup?url=` the contact behind a profile URL | `contacts.linkedin_lookup` |

## How a profile is read

1. LinkedIn's own JSON API (Voyager), as the logged-in user.
2. The JSON LinkedIn embeds in the profile pages.
3. The rendered "Show all" experience and education pages, whose text the
   server parses.

## Tests

```sh
node browser-extension/test-extract.js
```

It checks the Voyager parser, the checks that stop a batch on checkpoint,
login and rate-limit pages, and the pacing and payload helpers.
