// The setup wizard shown once after signing up (OnboardingFlow). Signing up
// leaves a marker here; the app opens the wizard once the new account has
// loaded, and finishing or skipping it clears the marker. The marker outlives
// a reload mid-wizard and the round trip through Google or Apple, but expires
// so an abandoned social sign up doesn't greet a later plain sign in.

const KEY = 'tt.onboarding'
const MAX_AGE = 24 * 60 * 60 * 1000

export function requestOnboarding() {
	try {
		localStorage.setItem(KEY, String(Date.now()))
	} catch {
		// Storage blocked: no wizard, the app works as usual.
	}
}

export function onboardingPending(): boolean {
	try {
		const at = Number(localStorage.getItem(KEY))
		return !!at && Date.now() - at < MAX_AGE
	} catch {
		return false
	}
}

export function finishOnboarding() {
	try {
		localStorage.removeItem(KEY)
	} catch {
		// nothing to clear
	}
}
