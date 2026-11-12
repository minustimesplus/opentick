# OpenTick for Android

A native Android app (Kotlin, Jetpack Compose) that looks and works like TickTick's Android app:

- **Tasks**: list cards grouped by list (Today) or date (Next 7 Days), the list's emoji as its icon, its color as an accent bar, swipe right to complete and left to delete (with undo).
- **Drawer**: Today, Inbox, Tags, folders and lists with counts, Completed, Won't Do and Trash. Smart lists are set to Show, Hide or Show if not empty in Settings → General, shared with the web and desktop apps.
- **Per list** (shared with the web and desktop apps): sort by date, priority, title, created time or the web's custom order; show or hide completed tasks; and for lists, a **List**, **Kanban** or **Timeline** layout. Kanban sections live on the server, so the board needs a connection. On the timeline, long press a bar and drag to move it, or drag its ends to change its dates.
- **Quick add** with smart recognition: `Call mom tomorrow 5pm #family !high ~Personal`.
- **Calendar** with TickTick's six views: List, Year, Month, Week (default), 3 Day and Day. Tasks with a start and end show as blocks; tap an empty slot to add a task there.
- **Eisenhower Matrix**, **Pomodoro**, **Search**.
- **Tab Bar**: pick and reorder the bottom tabs in Settings → Tab Bar, like TickTick.
- **Reminders** as local notifications, **share to OpenTick** from other apps, light and dark theme.

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

## Releases

The `Desktop release` workflow builds the APK on every push to `main` and
attaches `OpenTick-<version>.apk` to the same GitHub release as the desktop
apps. It signs with the permanent release key from these repository secrets:

| Secret | Value |
|---|---|
| `ANDROID_KEYSTORE_BASE64` | The keystore file, base64 encoded (`base64 -w0 opentick-release.jks`) |
| `ANDROID_KEYSTORE_PASSWORD` | Keystore password |
| `ANDROID_KEY_ALIAS` | Key alias |
| `ANDROID_KEY_PASSWORD` | Key password |

Without them the APK is signed with a throwaway debug key and the workflow
warns. Keep a backup of the keystore: if it's lost, new versions can't update
existing installs. The `versionCode` is the workflow run number, so every
release installs over the previous one.

## Updates

The app updates itself from GitHub releases (`update/Updater.kt`). Signed
releases carry `latest-android.json` (version, APK URL, SHA-256), which the app
reads from `releases/latest/download/latest-android.json`:

- It checks every 6 hours and when the app opens (at most every 6 hours), and from Settings → About → Check for Updates.
- Before installing, it checks the APK's hash, package name, version and that it's signed with the installed app's key.
- With **Install updates automatically** on (default) and Android 12+, updates
  install in the background without a prompt once OpenTick is the app's
  installer of record, i.e. after the first update installed through the app.
  That first update, and every update on Android 8–11, shows a notification
  and Android's install prompt. Android 14+ waits until the app isn't in use;
  Android 12–13 only installs while the app is in the background.
- Installing needs "Install unknown apps" allowed for OpenTick; Android asks the first time.

Debug builds don't check for updates. Releases signed with the debug key have
no feed, so installs never see them.

## Tests

```sh
./gradlew testDebugUnitTest                                        # parser tests
OPENTICK_TEST_SERVER=http://localhost:3456 ./gradlew testDebugUnitTest   # + end-to-end sync
```

Screenshots of the main screens, rendered on the JVM at a phone's size
(1080x2340, 440 dpi, font scale 1.1) with sample data, for comparing the UI
against TickTick:

```sh
./gradlew testDebugUnitTest --tests '*ScreenshotTest*' -Pscreenshots   # app/build/screenshots/*.png
```

The sync tests register throwaway users on the given server, so the server
needs local accounts with registration enabled. Point them at a dev server only.
