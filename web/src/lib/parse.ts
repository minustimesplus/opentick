// TickTick-style smart recognition for the quick-add bar.
// Supports (English + French):
//   dates:    today, tomorrow, day after tomorrow, weekday names, next week,
//             in 3 days, 26/10, 26 oct, oct 26
//   times:    3pm, 3:30pm, 15:00, 15h, 15h30, at 15:00, 14:00-16:00, 2pm-4pm
//   priority: !high !medium !low  (!h !m !l, !3 !2 !1, !!! !! , !haute !moyenne !faible)
//   tags:     #tag
//   list:     ~List  or  ^List

import {addDays, endOfDay, startOfDay, withMinutes} from './dates'
import type {Level} from './tasks'

export interface Parsed {
	title: string
	date: Date | null
	allDay: boolean
	end: Date | null
	priority: Level | null
	labels: string[]
	list: string | null
	/** Text fragments that were recognised, for highlighting. */
	matches: string[]
}

const WEEKDAYS: Record<string, number> = {
	sunday: 0, monday: 1, tuesday: 2, wednesday: 3, thursday: 4, friday: 5, saturday: 6,
	dimanche: 0, lundi: 1, mardi: 2, mercredi: 3, jeudi: 4, vendredi: 5, samedi: 6,
}

const MONTHS: Record<string, number> = {
	jan: 0, january: 0, janvier: 0, janv: 0,
	feb: 1, february: 1, fevrier: 1, février: 1, fev: 1, fév: 1, févr: 1,
	mar: 2, march: 2, mars: 2,
	apr: 3, april: 3, avril: 3, avr: 3,
	may: 4, mai: 4,
	jun: 5, june: 5, juin: 5,
	jul: 6, july: 6, juillet: 6, juil: 6,
	aug: 7, august: 7, aout: 7, août: 7,
	sep: 8, sept: 8, september: 8, septembre: 8,
	oct: 9, october: 9, octobre: 9,
	nov: 10, november: 10, novembre: 10,
	dec: 11, december: 11, decembre: 11, décembre: 11, déc: 11,
}

const MONTH_RE = Object.keys(MONTHS).sort((a, b) => b.length - a.length).join('|')
const WEEKDAY_RE = Object.keys(WEEKDAYS).join('|')

// A time token: 15:00, 15h, 15h30, 3pm, 3:30pm, 3 pm
const TIME = String.raw`(\d{1,2})(?:(?::|h)(\d{2})?)?\s*(am|pm)?`

function toMinutes(h: string, m: string | undefined, ap: string | undefined, hadMarker: boolean): number | null {
	let hour = parseInt(h, 10)
	const min = m ? parseInt(m, 10) : 0
	if (!hadMarker && !ap && !m) return null
	if (ap) {
		if (hour > 12) return null
		if (ap === 'pm' && hour < 12) hour += 12
		if (ap === 'am' && hour === 12) hour = 0
	}
	if (hour > 23 || min > 59) return null
	return hour * 60 + min
}

export function parseQuickAdd(input: string, now = new Date()): Parsed {
	let text = ' ' + input + ' '
	const matches: string[] = []
	const take = (re: RegExp): RegExpMatchArray | null => {
		const m = text.match(re)
		if (m) {
			matches.push(m[0].trim())
			text = text.replace(m[0], ' ')
		}
		return m
	}

	// priority
	let priority: Level | null = null
	const pm = take(/\s(!{1,3}|!(?:high|haute|h|3|medium|moyenne|moyen|m|2|low|faible|basse|l|1|none|aucune|0))(?=\s)/i)
	if (pm) {
		const v = pm[1].toLowerCase()
		if (/^!(high|haute|h|3)$|^!!!$/.test(v)) priority = 3
		else if (/^!(medium|moyenne|moyen|m|2)$|^!!$/.test(v)) priority = 2
		else if (/^!(low|faible|basse|l|1)$|^!$/.test(v)) priority = 1
		else priority = 0
	}

	// labels
	const labels: string[] = []
	let lm: RegExpMatchArray | null
	while ((lm = take(/\s#([\p{L}\p{N}_\-/]+)(?=\s)/u))) labels.push(lm[1])

	// list
	let list: string | null = null
	const listM = take(/\s[~^]([\p{L}\p{N}_\-]+)(?=\s)/u)
	if (listM) list = listM[1]

	// date
	let day: Date | null = null
	const today = startOfDay(now)
	let dm: RegExpMatchArray | null
	if ((dm = take(/\s(day after tomorrow|après-demain|apres-demain|après demain)(?=\s)/i))) {
		day = addDays(today, 2)
	} else if ((dm = take(/\s(today|tod|aujourd'hui|aujourd’hui|auj|tonight|ce soir)(?=\s)/i))) {
		day = today
	} else if ((dm = take(/\s(tomorrow|tmr|tmrw|demain)(?=\s)/i))) {
		day = addDays(today, 1)
	} else if ((dm = take(/\s(next week|semaine prochaine|la semaine prochaine)(?=\s)/i))) {
		const diff = (8 - today.getDay()) % 7 || 7
		day = addDays(today, diff)
	} else if ((dm = take(/\s(?:in|dans)\s+(\d{1,3})\s*(days?|jours?|weeks?|semaines?)(?=\s)/i))) {
		const n = parseInt(dm[1], 10) * (/^(w|s)/i.test(dm[2]) ? 7 : 1)
		day = addDays(today, n)
	} else if ((dm = take(new RegExp(String.raw`\s(?:(?:on|next|this|ce|le)\s+)?(${WEEKDAY_RE})(?:\s+(?:prochain|next))?(?=\s)`, 'i')))) {
		const target = WEEKDAYS[dm[1].toLowerCase()]
		let diff = (target - today.getDay() + 7) % 7
		if (diff === 0 || /next|prochain/i.test(dm[0])) diff = diff === 0 ? 7 : diff
		day = addDays(today, diff)
	} else if ((dm = take(/\s(\d{1,2})\/(\d{1,2})(?:\/(\d{2,4}))?(?=\s)/))) {
		const year = dm[3] ? (dm[3].length === 2 ? 2000 + parseInt(dm[3], 10) : parseInt(dm[3], 10)) : today.getFullYear()
		const d = new Date(year, parseInt(dm[2], 10) - 1, parseInt(dm[1], 10))
		if (!dm[3] && d < today) d.setFullYear(year + 1)
		day = d
	} else if ((dm = take(new RegExp(String.raw`\s(\d{1,2})(?:st|nd|rd|th|er)?\s+(${MONTH_RE})\.?(?:\s+(\d{4}))?(?=\s)`, 'i')))) {
		day = monthDay(parseInt(dm[1], 10), dm[2], dm[3], today)
	} else if ((dm = take(new RegExp(String.raw`\s(${MONTH_RE})\.?\s+(\d{1,2})(?:st|nd|rd|th)?(?:,?\s+(\d{4}))?(?=\s)`, 'i')))) {
		day = monthDay(parseInt(dm[2], 10), dm[1], dm[3], today)
	}

	// time / time range
	let startMin: number | null = null
	let endMin: number | null = null
	const range = text.match(new RegExp(String.raw`\s(?:at\s+|à\s+|a\s+|de\s+|from\s+)?${TIME}\s*(?:-|–|to|à|a)\s*${TIME}(?=\s)`, 'i'))
	if (range) {
		const s = toMinutes(range[1], range[2], range[3]?.toLowerCase() ?? range[6]?.toLowerCase(), /h|:/.test(range[0]) || !!range[3] || !!range[6])
		const e = toMinutes(range[4], range[5], range[6]?.toLowerCase(), /h|:/.test(range[0]) || !!range[6])
		if (s !== null && e !== null && e > s) {
			startMin = s
			endMin = e
			matches.push(range[0].trim())
			text = text.replace(range[0], ' ')
		}
	}
	if (startMin === null) {
		const tm = text.match(new RegExp(String.raw`\s(at\s+|à\s+|@)?${TIME}(?=\s)`, 'i'))
		if (tm) {
			const raw = tm[0]
			const marker = !!tm[1] || /:|h/i.test(raw.replace(tm[1] ?? '', ''))
			const minutes = toMinutes(tm[2], tm[3], tm[4]?.toLowerCase(), marker)
			if (minutes !== null) {
				startMin = minutes
				matches.push(raw.trim())
				text = text.replace(raw, ' ')
			}
		}
	}
	if (/\s(tonight|ce soir)\s/i.test(' ' + input + ' ') && startMin === null) startMin = 20 * 60

	let date: Date | null = null
	let end: Date | null = null
	let allDay = false
	if (startMin !== null) {
		let base = day ?? today
		// A bare time that already passed today means tomorrow.
		if (!day && startMin < now.getHours() * 60 + now.getMinutes()) base = addDays(today, 1)
		date = withMinutes(base, startMin)
		if (endMin !== null) end = withMinutes(base, endMin)
	} else if (day) {
		date = endOfDay(day)
		allDay = true
	}

	const title = text.replace(/\s+/g, ' ').trim()
	return {title: title || input.trim(), date, allDay, end, priority, labels, list, matches}
}

function monthDay(d: number, monthName: string, yearStr: string | undefined, today: Date): Date | null {
	const month = MONTHS[monthName.toLowerCase().replace('.', '')]
	if (month === undefined) return null
	const year = yearStr ? parseInt(yearStr, 10) : today.getFullYear()
	const r = new Date(year, month, d)
	if (!yearStr && r < today) r.setFullYear(year + 1)
	return r
}
