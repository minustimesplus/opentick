<script setup lang="ts">
import {computed, ref} from 'vue'
import {Check, Inbox, Menu} from 'lucide-vue-next'
import {useData} from '@/stores/data'
import {splitEmoji} from '@/lib/tasks'
import {t} from '@/lib/i18n'

const props = defineProps<{value?: number}>()
const emit = defineEmits<{select: [number]}>()
const data = useData()
const q = ref('')

const options = computed(() => {
	const inbox = data.inboxId ? [{id: data.inboxId, emoji: null as string | null, name: t('inbox'), inbox: true}] : []
	const lists = data.listProjects.map(p => ({id: p.id, ...splitEmoji(p.title), inbox: false}))
	return [...inbox, ...lists].filter(o => !q.value || o.name.toLowerCase().includes(q.value.toLowerCase()))
})
</script>

<template>
	<div class="menu picker">
		<input v-model="q" class="input search" :placeholder="t('search')" v-focus>
		<div class="scroll">
			<button v-for="o in options" :key="o.id" class="menu-item" @click="emit('select', o.id)">
				<Inbox v-if="o.inbox" :size="16" />
				<span v-else-if="o.emoji" class="emoji">{{ o.emoji }}</span>
				<Menu v-else :size="16" />
				<span class="name">{{ o.name }}</span>
				<Check v-if="o.id === props.value" :size="15" class="check" />
			</button>
		</div>
	</div>
</template>

<style scoped>
.picker {
	width: 240px;
}

.search {
	height: 30px;
	margin-bottom: 6px;
}

.scroll {
	max-height: 300px;
	overflow: auto;
}

.emoji {
	width: 16px;
	text-align: center;
}

.name {
	flex: 1;
	overflow: hidden;
	text-overflow: ellipsis;
}

.check {
	color: var(--accent);
}
</style>
