import {ref} from 'vue'

export interface Toast {
	id: number
	message: string
	action?: {label: string, run: () => void}
}

export const toasts = ref<Toast[]>([])
let next = 1

export function toast(message: string, action?: Toast['action'], timeout = 4000) {
	const id = next++
	toasts.value = [...toasts.value, {id, message, action}]
	setTimeout(() => dismiss(id), timeout)
}

export function dismiss(id: number) {
	toasts.value = toasts.value.filter(t => t.id !== id)
}
