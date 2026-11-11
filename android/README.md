# OpenTick for Android

A native Android app (Kotlin, Jetpack Compose) with a TickTick-style UI:

- **Tasks**: smart lists (Today, Tomorrow, Next 7 Days, Inbox, All, Completed), your lists and tags in the side drawer, date sections, swipe right to complete, swipe left to delete (with undo).
- **Quick add** with smart recognition: `Call mom tomorrow 5pm #family !high ~Personal`.
- **Task details**: title, description, due date and time, priority, list, tags, won't do.
- **Calendar** (month view), **Eisenhower Matrix**, **Focus** (Pomodoro) timer, light and dark theme.
- **Reminders** as local notifications at the due time (all-day tasks at 9:00), with a Complete action.
- **Share to OpenTick** from any app to create a task.

## Offline first

The app keeps everything in a local database and works fully offline. Changes
are queued and synced when the network is back: on launch, on pull to refresh,
shortly after every change and every 30 minutes in the background. If the same
task changed on both sides, the phone's edit wins for the fields the app edits
(title, description, done, due date, priority, list, tags).

"Use without an account" keeps tasks on the device only. Signing in later uploads them.

## Servers

The app works with **OpenTick Cloud** (the hosted instance) and **self-hosted** servers:

- Self-hosted servers with local accounts: sign in with username and password, or sign up.
- Any server, including the hosted one (Clerk sign in, Google, Apple), and SSO-only servers: **Sign in with browser**.
  The app opens the server's `/oauth/authorize` page and gets the session back via
  `vikunja-opentick://oauth/callback` (authorization code + PKCE, first-party client `opentick-android`).
  The server needs a web build that knows this client (`web/src/lib/oauthReturn.ts`).

Plain `http://` servers are allowed for home networks, as are user-installed CAs.
Typing an address without a scheme tries `https://` first, then `http://`.

## Building

Requirements: JDK 17 and the Android SDK (platform 35, build tools 35).

```sh
cd android
echo "sdk.dir=$HOME/android-sdk" > local.properties   # or set ANDROID_HOME
./gradlew assembleDebug      # app/build/outputs/apk/debug/app-debug.apk
./gradlew assembleRelease    # app/build/outputs/apk/release/app-release.apk
```

Release builds are signed with `keystore.properties` if present (not committed):

```properties
storeFile=opentick-release.jks
storePassword=...
keyAlias=opentick
keyPassword=...
```

Without it, release builds use the debug key, which is fine for installing on
your own phone. Updates must be signed with the same key as the installed app.

## Tests

```sh
./gradlew testDebugUnitTest                                        # parser tests
OPENTICK_TEST_SERVER=http://localhost:3456 ./gradlew testDebugUnitTest   # + end-to-end sync
```

The sync tests register throwaway users on the given server, so the server
needs local accounts with registration enabled. Point them at a dev server only.
