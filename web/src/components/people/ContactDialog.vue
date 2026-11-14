<script setup lang="ts">
import {computed, reactive, ref} from 'vue'
import Modal from '@/components/Modal.vue'
import {ApiError} from '@/api/client'
import type {Contact} from '@/api/types'
import {usePeople, type NewContactInput} from '@/stores/people'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

const props = defineProps<{groupId?: number}>()
const emit = defineEmits<{close: [], created: [contact: Contact]}>()
const people = usePeople()

const form = reactive({
	full_name: '',
	company: '',
	title: '',
	email: '',
	phone: '',
	location: '',
	linkedin_url: '',
})
const saving = ref(false)
const duplicate = ref(false)

const valid = computed(() => !!(form.full_name.trim() || form.email.trim() || form.linkedin_url.trim()))

async function save(allowDuplicate = false) {
	if (!valid.value || saving.value) return
	saving.value = true
	try {
		const input: NewContactInput = {...form}
		const contact = await people.create(input, allowDuplicate)
		if (props.groupId !== undefined) await people.setGroup(props.groupId, [contact.id], true)
		emit('created', contact)
		emit('close')
	} catch (e) {
		if (e instanceof ApiError && e.status === 409) duplicate.value = true
		toast((e as Error).message)
	} finally {
		saving.value = false
	}
}
</script>

<template>
	<Modal :width="460" @close="emit('close')">
		<form class="form" @submit.prevent="save(false)">
			<h3>{{ t('newContact') }}</h3>

			<input v-model="form.full_name" v-focus class="input" :placeholder="t('contactFullName')" maxlength="250">
			<div class="pair">
				<input v-model="form.title" class="input" :placeholder="t('contactTitle')" maxlength="250">
				<input v-model="form.company" class="input" :placeholder="t('contactCompany')" maxlength="250">
			</div>
			<div class="pair">
				<input v-model="form.email" class="input" type="email" :placeholder="t('email')" maxlength="250" @input="duplicate = false">
				<input v-model="form.phone" class="input" :placeholder="t('contactPhone')" maxlength="100">
			</div>
			<input v-model="form.location" class="input" :placeholder="t('contactLocation')" maxlength="250">
			<input
				v-model="form.linkedin_url"
				class="input"
				:placeholder="t('contactLinkedin') + ' URL'"
				maxlength="250"
				@input="duplicate = false"
			>

			<div v-if="duplicate" class="warn">{{ t('contactDuplicate') }}</div>

			<div class="actions">
				<div class="spacer" />
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button v-if="duplicate" type="button" class="btn" :aria-disabled="saving" @click="save(true)">
					{{ t('contactCreateAnyway') }}
				</button>
				<button v-else type="submit" class="btn primary" :aria-disabled="saving || !valid">{{ t('save') }}</button>
			</div>
		</form>
	</Modal>
</template>

<style scoped>
.form {
	display: flex;
	flex-direction: column;
	gap: 8px;
}

h3 {
	margin: 0 0 6px;
	font-size: 17px;
}

.pair {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 8px;
}

.warn {
	padding: 8px 10px;
	border-radius: 6px;
	background: color-mix(in srgb, var(--prio-medium) 14%, transparent);
	font-size: 13px;
}

.actions {
	display: flex;
	gap: 8px;
	margin-top: 10px;
}

.spacer {
	flex: 1;
}

.btn[aria-disabled="true"] {
	opacity: 0.5;
	cursor: default;
}
</style>
