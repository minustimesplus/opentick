package org.opentick.android.auth

import android.content.Context
import android.net.Uri
import androidx.browser.customtabs.CustomTabsIntent
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.opentick.android.data.AccountMode
import org.opentick.android.data.Api
import org.opentick.android.data.Prefs
import org.opentick.android.data.Repository
import org.opentick.android.data.ServerInfo
import org.opentick.android.sync.SyncWorker
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64

/**
 * Signing in. Self-hosted servers with local accounts take username and
 * password directly. Any server, including the hosted one (Clerk), also works
 * through the browser: the app opens /oauth/authorize, the user signs in on
 * the server's own page, and the server redirects back to
 * vikunja-opentick://oauth/callback with a code (authorization code + PKCE).
 */
class AuthManager(
    private val context: Context,
    private val prefs: Prefs,
    private val api: Api,
    private val repo: Repository,
) {
    // Kept in prefs: Android may kill the app while the browser is open.
    private val pending = context.getSharedPreferences("opentick_oauth", Context.MODE_PRIVATE)

    /** Finds the server. Without a scheme, https is tried first, then http (home servers). */
    suspend fun probe(serverInput: String): Pair<String, ServerInfo> = withContext(Dispatchers.IO) {
        val url = api.normalizeServerUrl(serverInput)
        val explicit = serverInput.trim().startsWith("http://") || serverInput.trim().startsWith("https://")
        try {
            url to api.info(url)
        } catch (e: java.io.IOException) {
            if (explicit || e is org.opentick.android.data.ApiException) throw e
            val http = "http://" + url.removePrefix("https://")
            http to api.info(http)
        }
    }

    suspend fun passwordLogin(server: String, username: String, password: String, totp: String?) = withContext(Dispatchers.IO) {
        val (token, refresh) = api.login(server, username.trim(), password, totp)
        finishSignIn(server, token, refresh)
    }

    suspend fun register(server: String, username: String, email: String, password: String) = withContext(Dispatchers.IO) {
        api.register(server, username.trim(), email.trim(), password)
        val (token, refresh) = api.login(server, username.trim(), password, null)
        finishSignIn(server, token, refresh)
    }

    /** Opens the server's sign in page in a browser tab. */
    fun startBrowserSignIn(server: String) {
        val verifier = randomString(32)
        val state = randomString(16)
        pending.edit().putString("verifier", verifier).putString("state", state).putString("server", server).commit()
        val challenge = base64url(MessageDigest.getInstance("SHA-256").digest(verifier.toByteArray()))
        val uri = Uri.parse("$server/oauth/authorize").buildUpon()
            .appendQueryParameter("response_type", "code")
            .appendQueryParameter("client_id", Api.OAUTH_CLIENT_ID)
            .appendQueryParameter("redirect_uri", Api.OAUTH_REDIRECT_URI)
            .appendQueryParameter("state", state)
            .appendQueryParameter("code_challenge", challenge)
            .appendQueryParameter("code_challenge_method", "S256")
            .build()
        val tab = CustomTabsIntent.Builder().setShowTitle(true).build()
        tab.intent.addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)
        tab.launchUrl(context, uri)
    }

    /** Handles vikunja-opentick://oauth/callback. Throws with a message on failure. */
    suspend fun handleCallback(uri: Uri) = withContext(Dispatchers.IO) {
        val server = pending.getString("server", null) ?: throw IllegalStateException("No sign in in progress. Please try again.")
        val verifier = pending.getString("verifier", null) ?: throw IllegalStateException("No sign in in progress. Please try again.")
        if (uri.getQueryParameter("state") != pending.getString("state", null)) throw IllegalStateException("Sign in response didn't match. Please try again.")
        // A code can only be used once; forget it whatever happens next.
        pending.edit().clear().apply()
        uri.getQueryParameter("error")?.let { throw IllegalStateException(if (it == "access_denied") "Sign in cancelled." else it) }
        val code = uri.getQueryParameter("code") ?: throw IllegalStateException("Sign in cancelled.")
        val res = api.exchangeCode(server, Api.OAUTH_CLIENT_ID, code, Api.OAUTH_REDIRECT_URI, verifier)
        finishSignIn(server, res.accessToken, res.refreshToken.ifBlank { null })
    }

    private suspend fun finishSignIn(server: String, token: String, refresh: String?) {
        val previousServer = prefs.serverUrl
        val wasLocal = prefs.mode.value == AccountMode.LOCAL
        // Data from another account must not leak into this one. Local-only
        // data (used without an account) is uploaded to the new account.
        if (!wasLocal && previousServer.isNotEmpty() && previousServer != server) repo.clearAll()
        prefs.serverUrl = server
        prefs.accessToken = token
        prefs.refreshToken = refresh
        prefs.setMode(AccountMode.SERVER)
        SyncWorker.schedulePeriodic(context)
        repo.sync()
    }

    fun useWithoutAccount() {
        prefs.setMode(AccountMode.LOCAL)
    }

    suspend fun signOut() {
        if (prefs.mode.value == AccountMode.SERVER) withContext(Dispatchers.IO) { api.logout() }
        SyncWorker.cancelAll(context)
        repo.clearAll()
        prefs.clearAccount()
    }

    private fun randomString(bytes: Int): String {
        val b = ByteArray(bytes)
        SecureRandom().nextBytes(b)
        return base64url(b)
    }

    private fun base64url(b: ByteArray) = Base64.getUrlEncoder().withoutPadding().encodeToString(b)
}
