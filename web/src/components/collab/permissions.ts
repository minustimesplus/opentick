import type {Permission} from '@/api/types'
import type {MessageKey} from '@/lib/i18n'

export const PERMISSIONS: {value: Permission, label: MessageKey}[] = [
	{value: 0, label: 'permRead'},
	{value: 1, label: 'permWrite'},
	{value: 2, label: 'permAdmin'},
]

export function permissionLabel(p: Permission): MessageKey {
	return PERMISSIONS.find(x => x.value === p)?.label ?? 'permRead'
}
