import { browser } from '$app/environment';

const STORAGE_KEY = 'dewey-settings';

export interface Settings {
	// Behavior
	pollIntervalMs: number;
	analyticsHistoryWindow: number;
	showPrometheusAlert: boolean;
	defaultSidebarOpen: boolean;
	// Alerts
	ramAlertThresholdMb: number;
	retryAlertThreshold: number;
	// Customization
	darkMode: boolean;
	accentColor: 'yellow' | 'blue' | 'green' | 'purple' | 'slate';
	chartColorTheme: 'default' | 'cool' | 'warm' | 'mono';
	layoutDensity: 'comfortable' | 'compact';
}

export const DEFAULTS: Settings = {
	pollIntervalMs: 5000,
	analyticsHistoryWindow: 40,
	showPrometheusAlert: true,
	defaultSidebarOpen: true,
	ramAlertThresholdMb: 200,
	retryAlertThreshold: 10,
	darkMode: false,
	accentColor: 'yellow',
	chartColorTheme: 'default',
	layoutDensity: 'comfortable'
};

// Full class strings must be spelled out so Tailwind's scanner doesn't purge them.
// hex is used for inline styles (sidebar border, topbar strip) where dynamic Tailwind classes aren't safe.
// text uses a darker shade in light mode and a lighter shade in dark mode so
// small uppercase headings clear WCAG AA (4.5:1) on both white cards and dark cards.
export const ACCENT = {
	yellow: { bg: 'bg-yellow-400', text: 'text-yellow-700 dark:text-yellow-400', ring: 'ring-yellow-400', hex: '#facc15' },
	blue:   { bg: 'bg-blue-400',   text: 'text-blue-700 dark:text-blue-400',     ring: 'ring-blue-400',   hex: '#60a5fa' },
	green:  { bg: 'bg-emerald-400', text: 'text-emerald-700 dark:text-emerald-400', ring: 'ring-emerald-400', hex: '#34d399' },
	purple: { bg: 'bg-purple-400', text: 'text-purple-700 dark:text-purple-400', ring: 'ring-purple-400', hex: '#c084fc' },
	slate:  { bg: 'bg-slate-500',  text: 'text-slate-700 dark:text-slate-400',   ring: 'ring-slate-400',  hex: '#64748b' }
} as const;

export const CHART_THEMES: Record<Settings['chartColorTheme'], string[]> = {
	default: [
		'#3b82f6', '#8b5cf6', '#6366f1', '#0ea5e9', '#10b981',
		'#059669', '#34d399', '#6ee7b7', '#f59e0b', '#ef4444',
		'#a78bfa', '#14b8a6', '#0d9488'
	],
	cool: [
		'#0ea5e9', '#06b6d4', '#14b8a6', '#0d9488', '#3b82f6',
		'#6366f1', '#8b5cf6', '#a78bfa', '#67e8f9', '#7dd3fc',
		'#93c5fd', '#c4b5fd', '#22d3ee'
	],
	warm: [
		'#f59e0b', '#f97316', '#ef4444', '#ec4899', '#fbbf24',
		'#fb923c', '#f87171', '#f472b6', '#fde68a', '#fca5a5',
		'#fdba74', '#f9a8d4', '#fcd34d'
	],
	mono: [
		'#1f2937', '#374151', '#4b5563', '#6b7280', '#9ca3af',
		'#d1d5db', '#111827', '#1f2937', '#374151', '#4b5563',
		'#6b7280', '#9ca3af', '#d1d5db'
	]
};

function load(): Settings {
	if (!browser) return { ...DEFAULTS };
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (raw) return { ...DEFAULTS, ...JSON.parse(raw) };
	} catch {
		// Corrupt storage — fall through to defaults
	}
	return { ...DEFAULTS };
}

// Exported (rather than kept private) so tests can build isolated
// instances instead of sharing the module-level `settings` singleton's
// localStorage-backed state across test cases (issue #240).
export function createSettingsStore() {
	let s = $state<Settings>(load());

	return {
		get value(): Settings {
			return s;
		},

		update(patch: Partial<Settings>) {
			s = { ...s, ...patch };
			if (browser) localStorage.setItem(STORAGE_KEY, JSON.stringify(s));
		},

		reset() {
			s = { ...DEFAULTS };
			if (browser) localStorage.removeItem(STORAGE_KEY);
		}
	};
}

// Singleton — all imports share the same reactive state instance
export const settings = createSettingsStore();
