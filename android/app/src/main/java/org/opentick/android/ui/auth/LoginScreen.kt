package org.opentick.android.ui.auth

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.outlined.OpenInBrowser
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import org.opentick.android.ui.MainViewModel
import org.opentick.android.ui.theme.Accent
import org.opentick.android.ui.theme.LocalExtra
import org.opentick.android.ui.theme.PrioHigh

const val CLOUD_URL = "https://opentick-b9732eb8fdac.herokuapp.com"

/**
 * Sign in: pick OpenTick Cloud or a self-hosted server. Servers with local
 * accounts get username + password; every server can also sign in through
 * the browser (needed for the hosted one, which uses Clerk).
 */
@Composable
fun LoginScreen(vm: MainViewModel, error: String?, onErrorShown: () -> Unit, allowLocal: Boolean = true, onCancel: (() -> Unit)? = null) {
    val scope = rememberCoroutineScope()
    val extra = LocalExtra.current
    var hosted by rememberSaveable { mutableStateOf(true) }
    var serverInput by rememberSaveable { mutableStateOf("") }
    var server by rememberSaveable { mutableStateOf<String?>(null) }
    var localEnabled by rememberSaveable { mutableStateOf(false) }
    var registration by rememberSaveable { mutableStateOf(false) }
    var registering by rememberSaveable { mutableStateOf(false) }
    var username by rememberSaveable { mutableStateOf("") }
    var email by rememberSaveable { mutableStateOf("") }
    var password by rememberSaveable { mutableStateOf("") }
    var totp by rememberSaveable { mutableStateOf("") }
    var needTotp by rememberSaveable { mutableStateOf(false) }
    var busy by rememberSaveable { mutableStateOf(false) }
    var message by rememberSaveable { mutableStateOf<String?>(null) }
    if (error != null) {
        message = error
        busy = false
        onErrorShown()
    }

    fun connect() {
        val input = if (hosted) CLOUD_URL else serverInput
        if (input.isBlank()) { message = "Enter your server's address"; return }
        busy = true; message = null
        scope.launch {
            try {
                val (url, i) = vm.auth.probe(input)
                server = url
                localEnabled = i.auth.local.enabled
                registration = i.auth.local.registrationEnabled
                // Hosted (Clerk) or SSO-only servers: go straight to the browser.
                if (!i.auth.local.enabled) vm.auth.startBrowserSignIn(url)
            } catch (e: Exception) {
                message = e.message ?: "Couldn't reach the server"
            } finally {
                busy = false
            }
        }
    }

    fun signIn() {
        val s = server ?: return
        busy = true; message = null
        scope.launch {
            try {
                if (registering) vm.auth.register(s, username, email, password)
                else vm.auth.passwordLogin(s, username, password, totp.ifBlank { null })
            } catch (e: Exception) {
                val m = e.message ?: "Sign in failed"
                if (m.contains("totp", true) || m.contains("two factor", true) || m.contains("passcode", true)) {
                    needTotp = true
                    message = if (totp.isBlank()) "Enter your two-factor code" else m
                } else message = m
            } finally {
                busy = false
            }
        }
    }

    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background).systemBarsPadding().imePadding()) {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 28.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Spacer(Modifier.height(56.dp))
            Box(Modifier.size(72.dp).clip(RoundedCornerShape(18.dp)).background(Accent), contentAlignment = Alignment.Center) {
                Icon(Icons.Filled.Check, null, tint = Color.White, modifier = Modifier.size(44.dp))
            }
            Spacer(Modifier.height(16.dp))
            Text("OpenTick", fontSize = 28.sp, fontWeight = FontWeight.Bold)
            Text("Stay organized. Online or offline.", color = extra.textTertiary)
            Spacer(Modifier.height(36.dp))

            if (server == null || !localEnabled) {
                SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                    SegmentedButton(hosted, { hosted = true; message = null }, SegmentedButtonDefaults.itemShape(0, 2)) { Text("OpenTick Cloud") }
                    SegmentedButton(!hosted, { hosted = false; message = null }, SegmentedButtonDefaults.itemShape(1, 2)) { Text("Self-hosted") }
                }
                Spacer(Modifier.height(16.dp))
                if (!hosted) {
                    OutlinedTextField(
                        serverInput, { serverInput = it }, label = { Text("Server address") },
                        placeholder = { Text("opentick.example.com") }, singleLine = true,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri),
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Spacer(Modifier.height(12.dp))
                } else {
                    Text(
                        "Sign in or create an account on the hosted service. You'll continue in your browser.",
                        fontSize = 13.sp, color = extra.textTertiary, textAlign = TextAlign.Center,
                    )
                    Spacer(Modifier.height(12.dp))
                }
                PrimaryButton(if (hosted) "Continue" else "Connect", busy) { connect() }
                if (server != null && !localEnabled) {
                    TextButton(onClick = { vm.auth.startBrowserSignIn(server!!) }) { Text("Open the sign in page again") }
                }
            } else {
                Text(server!!.removePrefix("https://").removePrefix("http://"), color = extra.textTertiary, fontSize = 13.sp)
                Spacer(Modifier.height(12.dp))
                OutlinedTextField(username, { username = it }, label = { Text("Username" + if (registering) "" else " or email") }, singleLine = true, modifier = Modifier.fillMaxWidth())
                if (registering) {
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(email, { email = it }, label = { Text("Email") }, singleLine = true,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email), modifier = Modifier.fillMaxWidth())
                }
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(
                    password, { password = it }, label = { Text("Password") }, singleLine = true,
                    visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                    modifier = Modifier.fillMaxWidth(),
                )
                if (needTotp && !registering) {
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(totp, { totp = it }, label = { Text("Two-factor code") }, singleLine = true,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword), modifier = Modifier.fillMaxWidth())
                }
                Spacer(Modifier.height(16.dp))
                PrimaryButton(if (registering) "Sign up" else "Sign in", busy) { signIn() }
                OutlinedButton(
                    onClick = { vm.auth.startBrowserSignIn(server!!) },
                    modifier = Modifier.fillMaxWidth().padding(top = 8.dp).height(48.dp),
                    shape = RoundedCornerShape(10.dp),
                ) {
                    Icon(Icons.Outlined.OpenInBrowser, null, Modifier.size(18.dp))
                    Spacer(Modifier.size(8.dp))
                    Text("Sign in with browser")
                }
                if (registration) {
                    TextButton(onClick = { registering = !registering; message = null }) {
                        Text(if (registering) "Have an account? Sign in" else "No account? Sign up")
                    }
                }
                TextButton(onClick = { server = null; message = null }) { Text("Use a different server") }
            }

            message?.let {
                Spacer(Modifier.height(12.dp))
                Text(it, color = PrioHigh, fontSize = 13.sp, textAlign = TextAlign.Center)
            }

            Spacer(Modifier.height(32.dp))
            if (allowLocal) {
                TextButton(onClick = { vm.auth.useWithoutAccount() }) {
                    Text("Use without an account", color = extra.textSecondary)
                }
            }
            if (onCancel != null) {
                TextButton(onClick = onCancel) { Text("Not now", color = extra.textSecondary) }
            }
            Spacer(Modifier.height(16.dp))
        }
    }
}

@Composable
private fun PrimaryButton(text: String, busy: Boolean, onClick: () -> Unit) {
    Button(
        onClick = onClick, enabled = !busy,
        modifier = Modifier.fillMaxWidth().height(48.dp),
        shape = RoundedCornerShape(10.dp),
        colors = ButtonDefaults.buttonColors(containerColor = Accent),
    ) {
        if (busy) CircularProgressIndicator(Modifier.size(20.dp), color = Color.White, strokeWidth = 2.dp)
        else Text(text, fontSize = 16.sp)
    }
}

