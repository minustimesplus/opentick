<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {ImageUp, Trash2, X} from 'lucide-vue-next'
import {loadContactPhoto} from '@/lib/contactPhoto'
import {t} from '@/lib/i18n'
import {usePeople} from '@/stores/people'

// Full-screen view of a contact's photo, with shortcuts to replace or remove it.
const props = defineProps<{
	contact: {id: number, name: string, has_photo: boolean}
}>()
const emit = defineEmits<{close: [], change: [], remove: []}>()

const people = usePeople()
const src = ref<string | null>(null)
const version = computed(() => people.photoVersions[props.contact.id] ?? 0)

watch(() => [props.contact.id, props.contact.has_photo, version.value] as const, async ([id, hasPhoto, v]) => {
	if (!hasPhoto) {
		emit('close')
		return
	}
	const url = await loadContactPhoto(id, v)
	if (props.contact.id === id && version.value === v) src.value = url
}, {immediate: true})

// Capture phase, so Escape closes only the viewer and not the panel behind it.
function onKey(e: KeyboardEvent) {
	if (e.key !== 'Escape') return
	e.stopPropagation()
	e.preventDefault()
	emit('close')
}
onMounted(() => window.addEventListener('keydown', onKey, true))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey, true))
</script>

<template>
	<Teleport to="body">
		<div class="photo-viewer" role="dialog" :aria-label="contact.name" @click.self="emit('close')">
			<button class="icon-btn close" :title="t('close')" @click="emit('close')"><X :size="22" /></button>
			<div class="frame" @click.self="emit('close')">
				<img v-if="src" :src="src" :alt="contact.name">
				<div v-else class="placeholder" />
				<div class="name">{{ contact.name }}</div>
				<div class="actions">
					<button class="action" @click="emit('change')"><ImageUp :size="16" /> {{ t('contactChangePhoto') }}</button>
					<button class="action danger" @click="emit('remove')"><Trash2 :size="16" /> {{ t('contactRemovePhoto') }}</button>
				</div>
			</div>
		</div>
	</Teleport>
</template>

<style scoped>
.photo-viewer {
	position: fixed;
	inset: 0;
	z-index: 1000;
	display: flex;
	align-items: center;
	justify-content: center;
	padding: 56px 16px calc(24px + env(safe-area-inset-bottom));
	background: rgb(0 0 0 / 85%);
	animation: fade 0.15s ease-out;
}

.close {
	position: absolute;
	top: calc(12px + env(safe-area-inset-top));
	right: 12px;
	color: #fff;
}

.frame {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 16px;
	max-width: 100%;
	max-height: 100%;
}

/* Photos are stored at up to 512px, so cap there to avoid blowing them up. */
img,
.placeholder {
	width: min(512px, 86vw, calc(100vh - 220px));
	height: min(512px, 86vw, calc(100vh - 220px));
	border-radius: 50%;
	object-fit: cover;
	background: rgb(255 255 255 / 8%);
	box-shadow: 0 10px 40px rgb(0 0 0 / 50%);
	animation: grow 0.18s ease-out;
}

.name {
	color: #fff;
	font-size: 18px;
	font-weight: 600;
	text-align: center;
}

.actions {
	display: flex;
	flex-wrap: wrap;
	justify-content: center;
	gap: 10px;
}

.action {
	display: inline-flex;
	align-items: center;
	gap: 7px;
	padding: 9px 16px;
	border-radius: 999px;
	background: rgb(255 255 255 / 14%);
	color: #fff;
	font-size: 14px;
	font-weight: 500;
}

.action:hover {
	background: rgb(255 255 255 / 22%);
}

.action.danger:hover {
	background: rgb(220 60 60 / 75%);
}

@keyframes fade {
	from { opacity: 0; }
}

@keyframes grow {
	from { transform: scale(0.85); }
}
</style>
