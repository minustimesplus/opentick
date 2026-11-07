import {computed} from 'vue'
import {useData} from '@/stores/data'
import {prefs} from './prefs'
import {SMART_LISTS, type SmartListDef, type SmartListKey} from './smartLists'

/** Resolves Settings > Smart List against the current data. */
export function useSmartLists() {
	const data = useData()

	/** Open-task count of a list, or null for lists without one. */
	function count(key: SmartListKey): number | null {
		const c = data.counts
		switch (key) {
			case 'all': return c.all
			case 'today': return c.today
			case 'tomorrow': return c.tomorrow
			case 'next7': return c.week
			case 'assigned': return c.assigned
			case 'inbox': return c.byProject.get(data.inboxId) ?? 0
			default: return null
		}
	}

	function nonEmpty(key: SmartListKey): boolean {
		switch (key) {
			case 'tags': return data.labels.length > 0
			case 'filters': return data.savedFilters.length > 0
			case 'completed': return !!data.closedNonEmpty.completed || data.allTasks.some(t => t.done && !t.wont_do)
			case 'wontDo': return !!data.closedNonEmpty.wontDo || data.allTasks.some(t => t.done && t.wont_do)
			case 'trash': return !!data.closedNonEmpty.trash
			case 'summary': return true
			default: return (count(key) ?? 0) > 0
		}
	}

	function visible(key: SmartListKey): boolean {
		const v = prefs.smartLists[key]
		return v === 'show' || (v === 'auto' && nonEmpty(key))
	}

	/** Asks the server about lists whose emptiness isn't known from synced data. */
	function probe() {
		for (const key of ['completed', 'wontDo', 'trash'] as const) {
			if (prefs.smartLists[key] === 'auto') data.probeNonEmpty(key)
		}
	}

	const shown = computed(() => {
		const by = (group: SmartListDef['group']) => SMART_LISTS.filter(d => d.group === group && visible(d.key))
		return {views: by('views'), closed: by('closed'), tags: visible('tags'), filters: visible('filters')}
	})

	return {count, visible, shown, probe}
}
