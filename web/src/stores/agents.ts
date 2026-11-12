// Handing tasks to AI agents: Claude (a Claude Code cloud session started
// through a scheduled task's API trigger) and Instinct (by email). Needs a
// server account; without one, everything here stays empty.

import {defineStore} from 'pinia'
import {computed, reactive, ref} from 'vue'
import {api, fetchAll} from '@/api/client'
import type {AgentConfig, AgentDispatch, AgentKind, ProjectAgentSettings, Task} from '@/api/types'
import {featureOn} from '@/lib/features'
import {useData} from './data'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

export const AGENTS: AgentKind[] = ['claude', 'instinct']

export function agentName(agent: AgentKind): string {
	return agent === 'claude' ? 'Claude' : 'Instinct'
}

export const useAgents = defineStore('agents', () => {
	const config = ref<AgentConfig | null>(null)
	const lists = reactive(new Map<number, ProjectAgentSettings>())
	/** Latest dispatch per task. */
	const latest = reactive(new Map<number, AgentDispatch>())
	let loading: Promise<void> | null = null

	const enabled = computed(() => featureOn('ai') && useData().authed && !useData().local)

	function ready(agent: AgentKind): boolean {
		const c = config.value
		if (!c) return false
		return agent === 'claude'
			? !!c.claude_fire_url && c.claude_token_set
			: !!c.instinct_email && c.mailer_enabled
	}

	async function load(): Promise<void> {
		if (!enabled.value) return
		loading ??= (async () => {
			try {
				const [cfg, settings, dispatches] = await Promise.all([
					api<AgentConfig>('GET', '/agents/config'),
					fetchAll<ProjectAgentSettings>('/agents/projects'),
					fetchAll<AgentDispatch>('/agents/dispatches'),
				])
				config.value = cfg
				lists.clear()
				for (const s of settings) lists.set(s.project_id, s)
				latest.clear()
				for (const d of dispatches) latest.set(d.task_id, d)
			} catch {
				// An older server without agent routes: the features stay hidden.
			} finally {
				loading = null
			}
		})()
		return loading
	}

	/** Forgets everything, e.g. on sign out, so the next account starts clean. */
	function clear() {
		config.value = null
		lists.clear()
		latest.clear()
	}

	async function saveConfig(patch: Partial<AgentConfig>): Promise<boolean> {
		try {
			config.value = await api<AgentConfig>('PUT', '/agents/config', {body: {...config.value, ...patch}})
			return true
		} catch (e) {
			toast((e as Error).message)
			return false
		}
	}

	/** Creates the private inbound address, or replaces it so the old one stops working. */
	async function rotateInboundAddress(): Promise<boolean> {
		try {
			config.value = await api<AgentConfig>('POST', '/agents/inbound-address')
			return true
		} catch (e) {
			toast((e as Error).message)
			return false
		}
	}

	function listSettings(projectId: number): ProjectAgentSettings {
		return lists.get(projectId) ?? {project_id: projectId, ai_context: '', agent: ''}
	}

	function listAgent(projectId: number): AgentKind | '' {
		return lists.get(projectId)?.agent ?? ''
	}

	async function saveListSettings(projectId: number, settings: {ai_context: string, agent: AgentKind | ''}): Promise<void> {
		const before = listSettings(projectId)
		if (before.ai_context === settings.ai_context.trim() && before.agent === settings.agent) return
		try {
			const saved = await api<ProjectAgentSettings>('PUT', `/projects/${projectId}/agent-settings`, {body: settings})
			lists.set(projectId, saved)
		} catch (e) {
			toast((e as Error).message)
		}
	}

	async function preview(taskId: number, extra: string): Promise<string> {
		const res = await api<{prompt: string}>('GET', `/tasks/${taskId}/agent-prompt`, {query: {extra: extra || undefined}})
		return res.prompt
	}

	async function send(taskId: number, agent: AgentKind, extra: string): Promise<AgentDispatch | null> {
		try {
			const d = await api<AgentDispatch>('POST', `/tasks/${taskId}/agent-dispatches`, {body: {agent, extra_prompt: extra}})
			latest.set(taskId, d)
			if (d.status === 'failed') {
				toast(t('agentSendFailed', {agent: agentName(agent), error: d.error}))
			} else if (d.session_url) {
				toast(t('agentSent', {agent: agentName(agent)}), {label: t('agentOpenSession'), run: () => window.open(d.session_url, '_blank', 'noopener')})
			} else {
				toast(t('agentSent', {agent: agentName(agent)}))
			}
			return d
		} catch (e) {
			toast((e as Error).message)
			return null
		}
	}

	async function history(taskId: number): Promise<AgentDispatch[]> {
		const all = await fetchAll<AgentDispatch>(`/tasks/${taskId}/agent-dispatches`)
		if (all[0]) latest.set(taskId, all[0])
		return all
	}

	/** New tasks in an agent list are sent on the server; pick up the result for the badge. */
	function afterTaskCreated(task: Task) {
		if (!enabled.value || !listAgent(task.project_id)) return
		for (const delay of [1500, 6000]) {
			window.setTimeout(() => {
				if (latest.has(task.id)) return
				history(task.id).catch(() => {})
			}, delay)
		}
	}

	return {config, lists, latest, enabled, ready, load, clear, saveConfig, rotateInboundAddress, listSettings, listAgent, saveListSettings, preview, send, history, afterTaskCreated}
})
