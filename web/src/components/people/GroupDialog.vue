<script setup lang="ts">
import {computed, ref} from 'vue'
import Modal from '@/components/Modal.vue'
import type {ContactGroup} from '@/api/types'
import {usePeople} from '@/stores/people'
import {SWATCHES} from '@/lib/tasks'
import {t} from '@/lib/i18n'

const props = defineProps<{group?: ContactGroup | null}>()
const emit = defineEmits<{close: [], saved: [group: ContactGroup], deleted: []}>()
const people = usePeople()

const COLORS = SWATCHES.filter(Boolean)

const name = ref(props.group?.name ?? '')
const color = ref((props.group?.hex_color ?? '').replace('#', ''))
const description = ref(props.group?.description ?? '')
const saving = ref(false)

const valid = computed(() => !!name.value.trim())

async function save() {
	if (!valid.value || saving.value) return
	saving.value = true
	const saved = await people.saveGroup({
		name: name.value.trim(),
		hex_color: color.value,
		description: description.value.trim(),
	}, props.group?.id)
	saving.value = false
	if (!saved) return
	emit('saved', saved)
	emit('close')
}

async function remove() {
	const group = props.group
	if (!group || !confirm(t('deleteGroupConfirm', {name: group.name}))) return
	if (await people.removeGroup(group.id)) {
		emit('deleted')
		emit('close')
	}
}
</script>

<template>
	<Modal @close="emit('close')">
		<form class="form" @submit.prevent="save">
			<h3>{{ group ? t('editGroup') : t('newGroup') }}</h3>

			<input v-model="name" v-focus class="input" :placeholder="t('groupNamePlaceholder')" maxlength="250" required>

			<label class="lbl">{{ t('color') }}</label>
			<div class="swatches">
				<button
					v-for="s in COLORS"
					:key="s"
					type="button"
					class="sw"
					:class="{on: color.toLowerCase() === s.toLowerCase()}"
					:style="{background: '#' + s}"
					@click="color = color.toLowerCase() === s.toLowerCase() ? '' : s"
				/>
			</div>

			<label class="lbl">{{ t('description') }}</label>
			<textarea v-model="description" class="input area" rows="3" />

			<div class="actions">
				<button v-if="group" type="button" class="btn danger" @click="remove">{{ t('delete') }}</button>
				<div class="spacer" />
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button type="submit" class="btn primary" :aria-disabled="saving || !valid">{{ t('save') }}</button>
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

.lbl {
	margin-top: 4px;
	font-size: 12px;
	color: var(--text-3);
}

.swatches {
	display: flex;
	flex-wrap: wrap;
	gap: 8px;
}

.sw {
	width: 22px;
	height: 22px;
	border-radius: 50%;
}

.sw.on {
	outline: 2px solid var(--text);
	outline-offset: 2px;
}

.area {
	height: auto;
	padding: 8px 10px;
	resize: vertical;
	font: inherit;
}

.actions {
	display: flex;
	gap: 8px;
	margin-top: 10px;
}

.spacer {
	flex: 1;
}

.btn.danger {
	border-color: transparent;
	color: var(--danger);
}

.btn[aria-disabled="true"] {
	opacity: 0.5;
	cursor: default;
}
</style>
