<script setup lang="ts">
import {ref} from 'vue'
import UserAvatar from '../UserAvatar.vue'
import {useData} from '@/stores/data'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'
import {buildCsvBackup, downloadBlob, NoTitleColumnError, startCsvImport, waitForCsvImport} from '@/lib/backup'

const data = useData()

const avatarInput = ref<HTMLInputElement | null>(null)
const importInput = ref<HTMLInputElement | null>(null)
const avatarBusy = ref(false)
const exporting = ref(false)
const importing = ref(false)

function errorText(e: unknown) {
	return (e as Error).message || t('error')
}

async function onAvatarPicked() {
	const file = avatarInput.value?.files?.[0]
	if (avatarInput.value) avatarInput.value.value = ''
	if (!file) return
	avatarBusy.value = true
	try {
		await data.uploadAvatar(file)
	} catch (e) {
		toast(errorText(e))
	} finally {
		avatarBusy.value = false
	}
}

async function removeAvatar() {
	avatarBusy.value = true
	try {
		await data.removeAvatar()
	} catch (e) {
		toast(errorText(e))
	} finally {
		avatarBusy.value = false
	}
}

async function exportBackup() {
	exporting.value = true
	try {
		const blob = await buildCsvBackup()
		downloadBlob(blob, `opentick-backup-${new Date().toISOString().slice(0, 10)}.csv`)
	} catch (e) {
		toast(errorText(e))
	} finally {
		exporting.value = false
	}
}

async function onImportPicked() {
	const file = importInput.value?.files?.[0]
	if (importInput.value) importInput.value.value = ''
	if (!file) return
	importing.value = true
	try {
		await startCsvImport(file)
		toast(t('backupImportStarted'))
		const status = await waitForCsvImport()
		if (!status) {
			toast(t('backupImportPending'))
		} else if (status.error_kind) {
			toast(status.error_message || t('backupImportFailed'))
		} else {
			await data.loadAll()
			toast(t('backupImportDone'))
		}
	} catch (e) {
		toast(e instanceof NoTitleColumnError ? t('backupNoTitle') : errorText(e))
	} finally {
		importing.value = false
	}
}
</script>

<template>
	<h2 class="section-title">{{ t('account') }}</h2>

	<div class="card profile">
		<UserAvatar :size="56" />
		<div class="who">
			<div class="name">{{ data.user?.name || data.user?.username }}</div>
			<div class="muted">{{ data.user?.email || data.user?.username }}</div>
		</div>
		<div class="actions">
			<button v-if="data.avatarUrl" class="btn" :disabled="avatarBusy" @click="removeAvatar">{{ t('remove') }}</button>
			<button class="btn" :disabled="avatarBusy" @click="avatarInput?.click()">{{ t('changeAvatar') }}</button>
		</div>
		<input ref="avatarInput" type="file" accept="image/png,image/jpeg,image/gif,image/bmp,image/tiff" hidden @change="onAvatarPicked">
	</div>

	<h2 class="section-title">{{ t('backup') }}</h2>

	<div class="card">
		<div class="row">
			<div>
				<div>{{ t('backupExport') }}</div>
				<div class="muted hint">{{ t('backupExportHint') }}</div>
			</div>
			<button class="btn" :disabled="exporting" @click="exportBackup">{{ exporting ? t('loading') : t('backupExportAction') }}</button>
		</div>
		<div class="row">
			<div>
				<div>{{ t('backupImport') }}</div>
				<div class="muted hint">{{ t('backupImportHint') }}</div>
			</div>
			<button class="btn" :disabled="importing" @click="importInput?.click()">{{ importing ? t('backupImporting') : t('backupImportAction') }}</button>
			<input ref="importInput" type="file" accept=".csv,text/csv" hidden @change="onImportPicked">
		</div>
	</div>
</template>

<style scoped>
.section-title {
	margin: 0 0 13px 15px;
	font-size: 14px;
	font-weight: 600;
}

.card {
	margin-bottom: 14px;
	padding: 2px 15px;
	border-radius: 8px;
	background: var(--settings-card);
}

.profile {
	display: flex;
	align-items: center;
	gap: 14px;
	padding: 14px 15px;
	margin-bottom: 24px;
}

.who {
	flex: 1;
	min-width: 0;
	font-size: 13px;
}

.who > div {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.name {
	margin-bottom: 2px;
	font-size: 15px;
	font-weight: 600;
}

.actions {
	display: flex;
	gap: 8px;
}

.row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 16px;
	min-height: 36px;
	padding: 10px 0;
	font-size: 13px;
}

.row + .row {
	border-top: 1px solid var(--border);
}

.hint {
	margin-top: 2px;
	font-size: 12px;
}

.btn:disabled {
	opacity: 0.6;
	pointer-events: none;
}
</style>
