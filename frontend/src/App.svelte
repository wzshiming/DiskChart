<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import { app } from '../wailsjs/go/models'
  import { asAdvice } from './lib/advice'
  import { api, onRulesChanged, onScanFinished, onScanProgress, onTreeChanged } from './lib/api'
  import { formatBytes, formatCount } from './lib/format'
  import { getLocale } from './lib/i18n'
  import { getUnit, setPlatform } from './lib/prefs.svelte'
  import { initialState, reduce, type UiEvent, type UiState } from './lib/state'
  import { onDarkChange, prefersDark } from './lib/theme'
  import TopBar from './components/TopBar.svelte'
  import StartPage from './components/StartPage.svelte'
  import ScanningPage from './components/ScanningPage.svelte'
  import ResultsPage from './components/ResultsPage.svelte'
  import RulesDialog from './components/RulesDialog.svelte'

  let ui: UiState = $state.raw(initialState)
  let dark = $state(prefersDark())
  let sameDevice = $state(true)
  let rulesOpen = $state(false)

  const summary = $derived(
    ui.summary
      ? { size: formatBytes(ui.summary.Size, getUnit()), files: formatCount(ui.summary.Files, getLocale()) }
      : null,
  )

  function dispatch(e: UiEvent) {
    ui = reduce(ui, e)
  }

  let crumbs = $state.raw<app.Crumb[]>([])
  const crumbKey = $derived(`${ui.phase}:${ui.currentID}:${ui.revision}`)
  let crumbSeq = 0
  $effect(() => {
    void crumbKey
    const { phase, currentID } = untrack(() => ui)
    const n = ++crumbSeq
    if (phase !== 'results' || currentID < 0) {
      crumbs = []
      return
    }
    api
      .crumbs(currentID)
      .then((c) => {
        if (n === crumbSeq) crumbs = c ?? []
      })
      .catch(() => {
        if (n === crumbSeq) crumbs = []
      })
  })

  async function startScan(root: string) {
    // Enter the scanning phase first: a tiny scan can finish before StartScan resolves.
    dispatch({ type: 'started', root })
    try {
      await api.startScan(root, { SameDevice: sameDevice })
    } catch (err) {
      ui = { ...reduce(ui, { type: 'reset' }), error: String(err) }
    }
  }

  function restore(s: app.ScanState) {
    if (ui.phase !== 'start' || ui.status !== 'idle') return
    if (s.Status === 'scanning') {
      dispatch({ type: 'started', root: s.Root })
      if (s.Progress) dispatch({ type: 'progress', progress: s.Progress })
    } else if ((s.Status === 'done' || s.Status === 'cancelled') && s.RootID >= 0) {
      dispatch({ type: 'started', root: s.Root })
      dispatch({
        type: 'finished',
        event: app.FinishedEvent.createFrom({
          Status: s.Status,
          Error: s.Error,
          Root: s.Root,
          RootID: s.RootID,
          Summary: s.Summary,
        }),
      })
      if (s.TreeStale) dispatch({ type: 'stale' })
    }
  }

  $effect(() => {
    document.documentElement.lang = getLocale() === 'zh' ? 'zh-CN' : 'en'
  })

  // New rules change advice without touching the tree, so the selected item is refreshed by hand.
  function rulesChanged() {
    dispatch({ type: 'rulesChanged' })
    const sel = ui.selected
    if (!sel) return
    api
      .subtree(sel.id, 0, 0, 0)
      .then((v) => {
        const cur = ui.selected
        if (!cur || cur.id !== v.ID) return
        dispatch({ type: 'select', item: { ...cur, advice: asAdvice(v.Advice), reason: v.Reason ?? '' } })
      })
      .catch(() => {})
  }

  onMount(() => {
    const offs = [
      onScanProgress((p) => dispatch({ type: 'progress', progress: p })),
      onScanFinished((e) => dispatch({ type: 'finished', event: e })),
      onTreeChanged((e) => dispatch({ type: 'treeChanged', event: e })),
      onRulesChanged(rulesChanged),
      onDarkChange((d) => (dark = d)),
    ]
    api
      .info()
      .then((i) => setPlatform(i.Platform))
      .catch(() => {})
    api.state().then(restore).catch(() => {})
    return () => offs.forEach((off) => off())
  })
</script>

<div class="flex h-full flex-col">
  <TopBar
    phase={ui.phase}
    root={ui.root}
    {crumbs}
    partial={ui.partial}
    {summary}
    onNavigate={(id) => dispatch({ type: 'navigate', id })}
    onBack={() => dispatch({ type: 'reset' })}
    onRules={() => (rulesOpen = true)}
  />
  <main class="min-h-0 flex-1">
    {#if ui.phase === 'start'}
      <StartPage error={ui.error} bind:sameDevice onScan={startScan} />
    {:else if ui.phase === 'scanning'}
      <ScanningPage progress={ui.progress} />
    {:else}
      <ResultsPage
        {ui}
        {dark}
        onNavigate={(id) => dispatch({ type: 'navigate', id })}
        onSelect={(item) => dispatch({ type: 'select', item })}
        onStale={() => dispatch({ type: 'stale' })}
        onRescan={() => startScan(ui.root)}
      />
    {/if}
  </main>
</div>

<RulesDialog open={rulesOpen} onClose={() => (rulesOpen = false)} />
