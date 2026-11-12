package org.opentick.android.update

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInfo
import android.content.pm.PackageInstaller
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.ProcessLifecycleOwner
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.OkHttpClient
import okhttp3.Request
import org.opentick.android.BuildConfig
import org.opentick.android.MainActivity
import org.opentick.android.R
import org.opentick.android.data.Prefs
import java.io.File
import java.security.MessageDigest
import java.util.concurrent.TimeUnit

/** latest-android.json, attached to every signed GitHub release by the release workflow. */
@Serializable
data class UpdateInfo(
    val versionCode: Int,
    val versionName: String,
    val url: String,
    val sha256: String,
    val size: Long = 0,
)

sealed interface UpdateState {
    data object Idle : UpdateState
    data object Checking : UpdateState
    data object UpToDate : UpdateState
    data class Available(val info: UpdateInfo) : UpdateState
    data class Downloading(val info: UpdateInfo, val percent: Int) : UpdateState
    data class Installing(val info: UpdateInfo) : UpdateState
    data class Failed(val message: String) : UpdateState
}

/**
 * Updates the sideloaded APK from GitHub releases.
 *
 * The app checks [FEED_URL], downloads a newer APK, checks its hash and that it
 * is signed with the same key as the running app, and hands it to
 * [PackageInstaller]. Once OpenTick is the app's installer of record (after the
 * first update through it), Android 12+ installs later updates without asking;
 * Android 14+ also waits until the app isn't in use.
 */
class Updater(private val context: Context, private val prefs: Prefs) {
    private val _state = MutableStateFlow<UpdateState>(UpdateState.Idle)
    val state: StateFlow<UpdateState> = _state
    private val lock = Mutex()
    private val http = OkHttpClient.Builder().readTimeout(60, TimeUnit.SECONDS).build()
    private val json = Json { ignoreUnknownKeys = true }

    init {
        val channel = NotificationChannel(CHANNEL, "App updates", NotificationManager.IMPORTANCE_DEFAULT)
        context.getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    /** Fetches the feed. Returns the update if it is newer than this build, else null. */
    suspend fun check(): UpdateInfo? = withContext(Dispatchers.IO) {
        if (!SUPPORTED) return@withContext null
        val current = _state.value
        if (current is UpdateState.Downloading || current is UpdateState.Installing) return@withContext null
        _state.value = UpdateState.Checking
        try {
            val req = Request.Builder().url(FEED_URL).header("Cache-Control", "no-cache").build()
            val info = http.newCall(req).execute().use { res ->
                // No feed on the latest release (e.g. built without the release key): nothing to install.
                if (res.code == 404) null
                else if (!res.isSuccessful) throw Exception("Update check failed (HTTP ${res.code})")
                else json.decodeFromString(UpdateInfo.serializer(), res.body!!.string())
            }
            prefs.lastUpdateCheck = System.currentTimeMillis()
            val newer = info?.takeIf { it.versionCode > BuildConfig.VERSION_CODE && it.url.startsWith("https://") }
            _state.value = if (newer != null) UpdateState.Available(newer) else UpdateState.UpToDate
            newer
        } catch (e: Exception) {
            _state.value = UpdateState.Failed(e.message ?: "Couldn't check for updates")
            null
        }
    }

    /** Last known update, from the latest check. */
    val available: UpdateInfo? get() = (_state.value as? UpdateState.Available)?.info

    /**
     * Whether an update can install with no prompt right now: the user allows it,
     * Android 12+, OpenTick installed the running version itself and may install
     * apps. Below Android 14 it also waits until the app is in the background,
     * since installing restarts it.
     */
    fun canInstallSilently(): Boolean {
        if (!prefs.autoUpdate || Build.VERSION.SDK_INT < Build.VERSION_CODES.S) return false
        if (!context.packageManager.canRequestPackageInstalls()) return false
        if (!isInstallerOfRecord()) return false
        return Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE || !appVisible()
    }

    private fun isInstallerOfRecord(): Boolean = runCatching {
        val src = context.packageManager.getInstallSourceInfo(context.packageName)
        val owner = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) src.updateOwnerPackageName else null
        src.installingPackageName == context.packageName && (owner == null || owner == context.packageName)
    }.getOrDefault(false)

    /** Downloads, verifies and installs [info]. With [silent], asks Android to skip the confirmation. */
    suspend fun install(info: UpdateInfo, silent: Boolean) = lock.withLock {
        withContext(Dispatchers.IO) {
            try {
                val apk = download(info)
                verify(apk, info)
                _state.value = UpdateState.Installing(info)
                commit(apk, info, silent)
            } catch (e: Exception) {
                _state.value = UpdateState.Failed(e.message ?: "Update failed")
                if (silent) notifyAvailable(info, force = true)
            }
        }
    }

    private fun download(info: UpdateInfo): File {
        val dir = File(context.cacheDir, "updates").apply { mkdirs() }
        val file = File(dir, "OpenTick-${info.versionCode}.apk")
        // Drop older downloads.
        dir.listFiles()?.filter { it != file }?.forEach { it.delete() }
        if (file.exists() && sha256(file).equals(info.sha256, ignoreCase = true)) return file

        _state.value = UpdateState.Downloading(info, 0)
        val tmp = File(dir, file.name + ".part")
        http.newCall(Request.Builder().url(info.url).build()).execute().use { res ->
            if (!res.isSuccessful) throw Exception("Download failed (HTTP ${res.code})")
            val body = res.body!!
            val total = body.contentLength().takeIf { it > 0 } ?: info.size
            body.byteStream().use { input ->
                tmp.outputStream().use { out ->
                    val buf = ByteArray(64 * 1024)
                    var done = 0L
                    var lastPct = -1
                    while (true) {
                        val n = input.read(buf)
                        if (n < 0) break
                        out.write(buf, 0, n)
                        done += n
                        val pct = if (total > 0) (done * 100 / total).toInt().coerceIn(0, 100) else 0
                        if (pct != lastPct) { lastPct = pct; _state.value = UpdateState.Downloading(info, pct) }
                    }
                }
            }
        }
        if (!sha256(tmp).equals(info.sha256, ignoreCase = true)) {
            tmp.delete()
            throw Exception("The download is damaged. Try again.")
        }
        tmp.renameTo(file)
        return file
    }

    /** The APK must be this app, the advertised version, and signed with this app's key. */
    private fun verify(apk: File, info: UpdateInfo) {
        val pm = context.packageManager
        val archive = archiveInfo(apk.path) ?: throw Exception("The download isn't a valid app.")
        if (archive.packageName != context.packageName) throw Exception("The download is a different app.")
        @Suppress("DEPRECATION")
        val code = if (Build.VERSION.SDK_INT >= 28) archive.longVersionCode else archive.versionCode.toLong()
        if (code != info.versionCode.toLong()) throw Exception("The download has an unexpected version.")
        val own = signers(ownInfo(pm), current = true)
        val theirs = signers(archive, current = false)
        if (own.isEmpty() || !theirs.containsAll(own)) {
            throw Exception("This update is signed with a different key and can't replace the installed app.")
        }
    }

    @Suppress("DEPRECATION")
    private fun archiveInfo(path: String): PackageInfo? =
        context.packageManager.getPackageArchiveInfo(path, SIGNING_FLAGS)

    @Suppress("DEPRECATION")
    private fun ownInfo(pm: PackageManager): PackageInfo = pm.getPackageInfo(context.packageName, SIGNING_FLAGS)

    /** SHA-256 of the signing certificates. For the archive, include rotated-from keys. */
    @Suppress("DEPRECATION")
    private fun signers(info: PackageInfo, current: Boolean): Set<String> {
        val sigs = if (Build.VERSION.SDK_INT >= 28) {
            val si = info.signingInfo ?: return emptySet()
            when {
                si.hasMultipleSigners() -> si.apkContentsSigners
                current -> si.apkContentsSigners
                else -> si.signingCertificateHistory
            }
        } else info.signatures
        return sigs.orEmpty().map { hex(MessageDigest.getInstance("SHA-256").digest(it.toByteArray())) }.toSet()
    }

    private fun commit(apk: File, info: UpdateInfo, silent: Boolean) {
        val installer = context.packageManager.packageInstaller
        // A previous attempt may still be waiting (e.g. for the app to go idle).
        for (s in installer.mySessions) runCatching { installer.abandonSession(s.sessionId) }

        val params = PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL).apply {
            setAppPackageName(context.packageName)
            setSize(apk.length())
            setInstallReason(PackageManager.INSTALL_REASON_USER)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                setRequireUserAction(
                    if (silent) PackageInstaller.SessionParams.USER_ACTION_NOT_REQUIRED
                    else PackageInstaller.SessionParams.USER_ACTION_REQUIRED,
                )
            }
        }
        val id = installer.createSession(params)
        try {
            installer.openSession(id).use { session ->
                session.openWrite("base.apk", 0, apk.length()).use { out ->
                    apk.inputStream().use { it.copyTo(out) }
                    session.fsync(out)
                }
                val status = UpdateReceiver.statusIntent(context, id, info, silent)
                if (silent && Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
                    // Install once the app isn't in the foreground or being used, within a day.
                    installer.commitSessionAfterInstallConstraintsAreMet(
                        id, status, android.content.pm.PackageInstaller.InstallConstraints.GENTLE_UPDATE,
                        TimeUnit.HOURS.toMillis(24),
                    )
                } else {
                    session.commit(status)
                }
            }
        } catch (e: Exception) {
            runCatching { installer.abandonSession(id) }
            throw e
        }
    }

    /** Called with the installer's result (see [UpdateReceiver]). */
    internal fun onInstallFailed(message: String) {
        _state.value = UpdateState.Failed(message)
    }

    // ─── Notifications ─────────────────────────────────────────────────

    /** "Update available", once per version unless [force]. Tapping it installs (see MainActivity). */
    fun notifyAvailable(info: UpdateInfo, force: Boolean = false) {
        if (!force && prefs.notifiedUpdate == info.versionCode) return
        prefs.notifiedUpdate = info.versionCode
        val open = PendingIntent.getActivity(
            context, NOTIFICATION_ID,
            Intent(context, MainActivity::class.java).putExtra(MainActivity.EXTRA_INSTALL_UPDATE, true)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        notify("OpenTick ${info.versionName} is available", "Tap to update.", open)
    }

    internal fun notify(title: String, text: String, intent: PendingIntent?) {
        val n = NotificationCompat.Builder(context, CHANNEL)
            .setSmallIcon(R.drawable.ic_notification)
            .setColor(0xFF4772FA.toInt())
            .setContentTitle(title)
            .setContentText(text)
            .setContentIntent(intent)
            .setAutoCancel(true)
            .build()
        runCatching { NotificationManagerCompat.from(context).notify(NOTIFICATION_ID, n) }
    }

    internal fun cancelNotification() = NotificationManagerCompat.from(context).cancel(NOTIFICATION_ID)

    companion object {
        const val FEED_URL = "https://github.com/minustimesplus/opentick/releases/latest/download/latest-android.json"
        const val CHANNEL = "updates"
        private const val NOTIFICATION_ID = 0x0FE7

        /** Debug builds have their own package and key, so there is nothing to update them with. */
        val SUPPORTED = !BuildConfig.DEBUG

        @Suppress("DEPRECATION")
        private val SIGNING_FLAGS =
            if (Build.VERSION.SDK_INT >= 28) PackageManager.GET_SIGNING_CERTIFICATES else PackageManager.GET_SIGNATURES

        fun appVisible(): Boolean =
            ProcessLifecycleOwner.get().lifecycle.currentState.isAtLeast(Lifecycle.State.STARTED)

        private fun sha256(file: File): String {
            val md = MessageDigest.getInstance("SHA-256")
            file.inputStream().use { input ->
                val buf = ByteArray(64 * 1024)
                while (true) { val n = input.read(buf); if (n < 0) break; md.update(buf, 0, n) }
            }
            return hex(md.digest())
        }

        private fun hex(bytes: ByteArray) = bytes.joinToString("") { "%02x".format(it) }
    }
}
