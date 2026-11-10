<script setup lang="ts">
// Shown from the avatar when there is no account (local mode and the demo):
// sign up on the hosted instance, or self-host.
import {ref} from 'vue'
import {useRouter} from 'vue-router'
import {ChartPie, Cloud, Copy, ExternalLink, LogIn, RotateCcw, Server, Settings} from 'lucide-vue-next'
import Modal from './Modal.vue'
import {useData} from '@/stores/data'
import {t} from '@/lib/i18n'
import {openSettings} from '@/lib/ui'
import {toast} from '@/lib/toast'
import {DEMO, HOSTED_SIGNUP_URL, REPO_URL, SELF_HOST_DOCS_URL} from '@/lib/localMode'

const emit = defineEmits<{close: []}>()
const data = useData()
const router = useRouter()
const showSteps = ref(false)

const dockerCmd = `git clone ${REPO_URL}.git opentick
cd opentick
docker build -t opentick .
docker run -d --name opentick -p 3456:3456 \\
  -v opentick-db:/db -v opentick-files:/app/vikunja/files \\
  opentick`

async function copyCmd() {
	try {
		await navigator.clipboard.writeText(dockerCmd)
		toast(t('copied'))
	} catch {
		// Clipboard blocked; the command is still selectable.
	}
}

function go(fn: () => void) {
	emit('close')
	fn()
}

async function resetData() {
	if (!confirm(t('resetExampleDataConfirm'))) return
	const {resetDB} = await import('@/api/local/db')
	resetDB()
	// Every store caches what it loaded; a reload starts them all fresh.
	window.location.reload()
}

async function signIn() {
	emit('close')
	await data.logout()
}
</script>

<template>
	<Modal :width="480" @close="emit('close')">
		<div class="local">
			<div class="head">
				<Cloud :size="22" />
				<h2>{{ t('localTitle') }}</h2>
			</div>
			<p class="intro">{{ DEMO ? t('demoIntro') : t('localIntro') }}</p>

			<a v-if="HOSTED_SIGNUP_URL" class="option" :href="HOSTED_SIGNUP_URL" target="_blank" rel="noopener">
				<span class="icon cloud"><Cloud :size="20" /></span>
				<span class="text">
					<strong>{{ t('hostedTitle') }}</strong>
					<span>{{ t('hostedDesc') }}</span>
				</span>
				<span class="action">{{ t('hostedAction') }} <ExternalLink :size="13" /></span>
			</a>

			<div class="option" :class="{open: showSteps}" role="button" tabindex="0" @click="showSteps = !showSteps" @keydown.enter="showSteps = !showSteps">
				<span class="icon server"><Server :size="20" /></span>
				<span class="text">
					<strong>{{ t('selfHostTitle') }}</strong>
					<span>{{ t('selfHostDesc') }}</span>
				</span>
				<span v-if="!showSteps" class="action">{{ t('selfHostAction') }}</span>
			</div>

			<ol v-if="showSteps" class="steps">
				<li>
					<span class="step-head">
						{{ t('selfHostStep1') }}
						<button class="copy" type="button" @click="copyCmd"><Copy :size="12" /> {{ t('copy') }}</button>
					</span>
					<pre class="cmd"><code>{{ dockerCmd }}</code></pre>
				</li>
				<li>{{ t('selfHostStep2') }}</li>
				<li>{{ t('selfHostStep3') }}</li>
				<li class="guide"><a :href="SELF_HOST_DOCS_URL" target="_blank" rel="noopener">{{ t('selfHostGuide') }} <ExternalLink :size="12" /></a></li>
			</ol>

			<div class="foot">
				<button class="foot-btn" @click="go(openSettings)"><Settings :size="14" /> {{ t('settings') }}</button>
				<button class="foot-btn" @click="go(() => router.push('/statistics'))"><ChartPie :size="14" /> {{ t('statistics') }}</button>
				<button class="foot-btn" @click="resetData"><RotateCcw :size="14" /> {{ t('resetExampleData') }}</button>
				<button v-if="!DEMO" class="foot-btn" @click="signIn"><LogIn :size="14" /> {{ t('signInToServer') }}</button>
			</div>
		</div>
	</Modal>
</template>

<style scoped>
.local {
	display: flex;
	flex-direction: column;
	gap: 12px;
}

.head {
	display: flex;
	align-items: center;
	gap: 10px;
	color: var(--accent);
}

h2 {
	margin: 0;
	font-size: 17px;
	color: var(--text);
}

.intro {
	margin: 0 0 4px;
	color: var(--text-2);
	font-size: 13px;
	line-height: 1.5;
}

.option {
	display: flex;
	align-items: center;
	gap: 12px;
	padding: 12px;
	border: 1px solid var(--border-strong);
	border-radius: 10px;
	color: inherit;
	text-decoration: none;
	cursor: pointer;
}

.option:hover,
.option.open {
	border-color: var(--accent);
	background: var(--accent-soft);
}

.icon {
	display: flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	width: 38px;
	height: 38px;
	border-radius: 9px;
	color: #fff;
}

.icon.cloud {
	background: var(--accent);
}

.icon.server {
	background: #40c057;
}

.text {
	display: flex;
	flex-direction: column;
	gap: 2px;
	flex: 1;
	min-width: 0;
	font-size: 12.5px;
	color: var(--text-2);
	line-height: 1.4;
}

.text strong {
	font-size: 14px;
	color: var(--text);
}

.action {
	display: flex;
	align-items: center;
	gap: 4px;
	flex-shrink: 0;
	color: var(--accent);
	font-size: 13px;
	font-weight: 500;
}

.steps {
	margin: 0;
	padding-left: 22px;
	font-size: 13px;
	line-height: 1.5;
	color: var(--text);
}

.steps li + li {
	margin-top: 6px;
}

.steps .guide {
	list-style: none;
	margin-left: -22px;
}

.steps a {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	color: var(--accent);
}

.step-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
}

.cmd {
	margin: 6px 0 0;
	padding: 10px 12px;
	border-radius: 8px;
	background: var(--bg-input);
	font-size: 11.5px;
	line-height: 1.5;
	overflow-x: auto;
}

.copy {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	font-size: 12px;
	color: var(--accent);
}

.foot {
	display: flex;
	flex-wrap: wrap;
	gap: 4px 12px;
	padding-top: 10px;
	border-top: 1px solid var(--border);
}

.foot-btn {
	display: inline-flex;
	align-items: center;
	gap: 5px;
	font-size: 12.5px;
	color: var(--text-2);
}

.foot-btn:hover {
	color: var(--text);
}
</style>
