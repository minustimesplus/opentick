import type {FocusSound} from './prefs'

/**
 * White noise for the focus timer, synthesized with Web Audio so the app
 * ships no audio files: a few seconds of noise looped, shaped by filters.
 */

type Ambient = Exclude<FocusSound, 'none'>

let ctx: AudioContext | null = null
let gain: GainNode | null = null
let nodes: AudioNode[] = []
let current: Ambient | null = null
let tickTimer: number | undefined

function noiseBuffer(c: AudioContext, color: 'white' | 'pink' | 'brown', seconds = 4): AudioBuffer {
	const length = c.sampleRate * seconds
	const buffer = c.createBuffer(2, length, c.sampleRate)
	for (let ch = 0; ch < 2; ch++) {
		const data = buffer.getChannelData(ch)
		// Pink: Paul Kellet's filter; brown: integrated white noise.
		let b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0, last = 0
		for (let i = 0; i < length; i++) {
			const white = Math.random() * 2 - 1
			if (color === 'white') {
				data[i] = white * 0.5
			} else if (color === 'pink') {
				b0 = 0.99886 * b0 + white * 0.0555179
				b1 = 0.99332 * b1 + white * 0.0750759
				b2 = 0.969 * b2 + white * 0.153852
				b3 = 0.8665 * b3 + white * 0.3104856
				b4 = 0.55 * b4 + white * 0.5329522
				b5 = -0.7616 * b5 - white * 0.016898
				data[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + white * 0.5362) * 0.11
				b6 = white * 0.115926
			} else {
				last = (last + 0.02 * white) / 1.02
				data[i] = last * 3.5
			}
		}
	}
	return buffer
}

function loop(c: AudioContext, buffer: AudioBuffer): AudioBufferSourceNode {
	const src = c.createBufferSource()
	src.buffer = buffer
	src.loop = true
	src.start()
	return src
}

function build(c: AudioContext, out: AudioNode, sound: Ambient) {
	if (sound === 'white' || sound === 'pink' || sound === 'brown') {
		const src = loop(c, noiseBuffer(c, sound))
		src.connect(out)
		nodes.push(src)
		return
	}
	if (sound === 'rain') {
		// Pink noise with the lows cut sounds like steady rain.
		const src = loop(c, noiseBuffer(c, 'pink'))
		const hp = c.createBiquadFilter()
		hp.type = 'highpass'
		hp.frequency.value = 400
		const lp = c.createBiquadFilter()
		lp.type = 'lowpass'
		lp.frequency.value = 7000
		src.connect(hp).connect(lp).connect(out)
		nodes.push(src, hp, lp)
		return
	}
	if (sound === 'ocean') {
		// Brown noise swelling and falling every eight seconds or so.
		const src = loop(c, noiseBuffer(c, 'brown'))
		const swell = c.createGain()
		swell.gain.value = 0.55
		const lfo = c.createOscillator()
		lfo.frequency.value = 0.12
		const depth = c.createGain()
		depth.gain.value = 0.45
		lfo.connect(depth).connect(swell.gain)
		lfo.start()
		src.connect(swell).connect(out)
		nodes.push(src, swell, lfo, depth)
		return
	}
	// ticking: a short click every second, like a mechanical kitchen timer.
	const click = () => {
		if (!ctx || !gain) return
		const osc = ctx.createOscillator()
		const env = ctx.createGain()
		osc.type = 'square'
		osc.frequency.value = 2400
		env.gain.setValueAtTime(0.0001, ctx.currentTime)
		env.gain.exponentialRampToValueAtTime(0.5, ctx.currentTime + 0.002)
		env.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.03)
		osc.connect(env).connect(gain)
		osc.start()
		osc.stop(ctx.currentTime + 0.04)
	}
	click()
	tickTimer = window.setInterval(click, 1000)
}

function teardown() {
	clearInterval(tickTimer)
	tickTimer = undefined
	for (const n of nodes) {
		try {
			if (n instanceof AudioScheduledSourceNode) n.stop()
			n.disconnect()
		} catch {
			// already stopped
		}
	}
	nodes = []
	current = null
}

/** Plays `sound` at `volume` (0..1), replacing whatever was playing. */
export function playAmbient(sound: Ambient, volume: number) {
	try {
		if (!ctx) {
			ctx = new AudioContext()
			gain = ctx.createGain()
			gain.connect(ctx.destination)
		}
		if (ctx.state === 'suspended') ctx.resume().catch(() => {})
		gain!.gain.setTargetAtTime(Math.max(0, Math.min(1, volume)) * 0.6, ctx.currentTime, 0.05)
		if (current === sound) return
		teardown()
		build(ctx, gain!, sound)
		current = sound
	} catch {
		// no audio output
	}
}

export function stopAmbient() {
	teardown()
	if (ctx && ctx.state === 'running') ctx.suspend().catch(() => {})
}
