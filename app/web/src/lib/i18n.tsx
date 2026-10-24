import { useMemo } from 'react'
import { createI18n, type I18n as BaseI18n } from '@mqtt-home/ui/i18n'

export { LANGS, LangSwitch, resolveText, type Lang, type Text } from '@mqtt-home/ui/i18n'

// German is the house language; English stays one tap away. Per device.
const dict = {
  de: {
    title: 'Trockner',
    'nav.overview': 'Übersicht',
    'nav.programs': 'Programme',
    disconnected: 'Keine Verbindung',
    export: 'Läufe exportieren',
    import: 'Läufe importieren',
    'import.done': '{n} Läufe importiert ({skipped} übersprungen).',
    'import.failed': 'Import fehlgeschlagen: {error}',
    'state.idle': 'Bereit',
    'state.running': 'Läuft',
    'state.cooling': 'Kühlt ab',
    'state.antiCrease': 'Knitterschutz',
    'state.offline': 'Offline',
    remaining: 'Restzeit',
    'eta.at': 'fertig um {time}',
    'antiCrease.hint': 'Fertig – die Wäsche kann entnommen werden.',
    detecting: 'Programm wird erkannt …',
    program: 'Programm',
    power: 'Leistung',
    energy: 'Energie',
    elapsed: 'Laufzeit',
    progress: 'Fortschritt',
    'idle.text': 'Der Trockner ist aus.',
    'idle.draw': 'Aktuelle Leistung: {power}',
    runs: 'Letzte Läufe',
    'runs.count': '{n} Läufe',
    'runs.none': 'Noch keine Läufe aufgezeichnet.',
    unlabeled: 'Ohne Namen',
    labeled: 'Benannt',
    'programs.none': 'Noch keine Programme gelernt. Nach ein paar Läufen, die du benennst, erkennt der Trockner sie selbst.',
    'programs.runs': '{n} Läufe',
    'programs.runs1': '1 Lauf',
    'programs.auto': 'Automatisch erkannt',
    'programs.autoHint': 'Automatisch gefundene Gruppe – öffne einen passenden Lauf und gib ihm einen Namen, um sie zu trainieren.',
    typical: 'Typisch',
    peak: 'Spitze',
    average: 'Mittel',
    duration: 'Dauer',
    back: 'Zurück',
    delete: 'Löschen',
    'delete.confirm': 'Diesen Lauf löschen? Das lässt sich nicht rückgängig machen.',
    loading: 'Wird geladen …',
    'run.unlabeled': 'Lauf ohne Namen',
    'run.auto': 'Automatisch erkannt: {program}',
    'run.match': '{pct} % Übereinstimmung',
    'label.title': 'Programmname',
    'label.placeholder': 'z. B. Baumwolle, Pflegeleicht, Eco …',
    save: 'Speichern',
    clear: 'Entfernen',
    'phase.drying': 'Trocknen',
    'phase.cooling': 'Abkühlen',
    'phase.anti-crease': 'Knitterschutz',
  },
  en: {
    title: 'Dryer',
    'nav.overview': 'Overview',
    'nav.programs': 'Programs',
    disconnected: 'Disconnected',
    export: 'Export runs',
    import: 'Import runs',
    'import.done': 'Imported {n} runs ({skipped} skipped).',
    'import.failed': 'Import failed: {error}',
    'state.idle': 'Idle',
    'state.running': 'Drying',
    'state.cooling': 'Cooling down',
    'state.antiCrease': 'Anti-crease',
    'state.offline': 'Offline',
    remaining: 'Remaining',
    'eta.at': 'done at {time}',
    'antiCrease.hint': 'Done – the laundry can be taken out.',
    detecting: 'Detecting program …',
    program: 'Program',
    power: 'Power',
    energy: 'Energy',
    elapsed: 'Elapsed',
    progress: 'Progress',
    'idle.text': 'The dryer is off.',
    'idle.draw': 'Current draw: {power}',
    runs: 'Recent runs',
    'runs.count': '{n} runs',
    'runs.none': 'No runs recorded yet.',
    unlabeled: 'Unlabeled',
    labeled: 'Labeled',
    'programs.none': 'No programs learned yet. Finish a few runs and label them to train recognition.',
    'programs.runs': '{n} runs',
    'programs.runs1': '1 run',
    'programs.auto': 'Auto-detected',
    'programs.autoHint': 'Auto-detected cluster – open a matching run and give it a name to train it.',
    typical: 'Typical',
    peak: 'Peak',
    average: 'Average',
    duration: 'Duration',
    back: 'Back',
    delete: 'Delete',
    'delete.confirm': 'Delete this run? This cannot be undone.',
    loading: 'Loading …',
    'run.unlabeled': 'Unlabeled run',
    'run.auto': 'Auto-detected: {program}',
    'run.match': '{pct} % match',
    'label.title': 'Program name',
    'label.placeholder': 'e.g. Cottons, Synthetics, Eco …',
    save: 'Save',
    clear: 'Clear',
    'phase.drying': 'Drying',
    'phase.cooling': 'Cooling',
    'phase.anti-crease': 'Anti-crease',
  },
}

export type Key = keyof (typeof dict)['de']

export interface I18n extends BaseI18n<Key> {
  /** "1 h 23 min", "12 min 5 s". */
  dur: (sec: number) => string
  watts: (w: number) => string
  energy: (wh: number) => string
  /** 24h clock time, "14:05". */
  clock: (iso?: string) => string
  /** "Sa., 18. Okt., 14:05". */
  dateTime: (iso?: string) => string
  phase: (p: string) => string
}

const base = createI18n(dict)
export const I18nProvider = base.I18nProvider

export function useI18n(): I18n {
  const i = base.useI18n()
  return useMemo<I18n>(() => {
    const { locale, t, num } = i
    const dur: I18n['dur'] = (sec) => {
      if (sec < 0 || !isFinite(sec)) return '–'
      const s = Math.round(sec)
      const h = Math.floor(s / 3600)
      const m = Math.floor((s % 3600) / 60)
      if (h > 0) return m > 0 ? `${h} h ${m} min` : `${h} h`
      // Seconds are noise once a run is past ten minutes.
      if (m >= 10) return `${m} min`
      if (m > 0) return `${m} min ${s % 60} s`
      return `${s} s`
    }
    const watts: I18n['watts'] = (w) => (w >= 1000 ? `${num(w / 1000, 2)} kW` : `${num(w)} W`)
    const energy: I18n['energy'] = (wh) => (wh >= 1000 ? `${num(wh / 1000, 2)} kWh` : `${num(wh)} Wh`)
    const clock: I18n['clock'] = (iso) =>
      iso ? new Date(iso).toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit', hour12: false }) : '–'
    const dateTime: I18n['dateTime'] = (iso) =>
      iso
        ? new Date(iso).toLocaleString(locale, {
            weekday: 'short',
            day: 'numeric',
            month: 'short',
            hour: '2-digit',
            minute: '2-digit',
            hour12: false,
          })
        : '–'
    const phase: I18n['phase'] = (p) => {
      const k = `phase.${p}` as Key
      return k in dict[i.lang] ? t(k) : p
    }
    return { ...i, dur, watts, energy, clock, dateTime, phase }
  }, [i])
}
