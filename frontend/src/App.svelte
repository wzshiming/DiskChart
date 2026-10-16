<script lang="ts">
  import { onMount } from 'svelte'
  import { app } from '../wailsjs/go/models'
  import { api, onScanFinished, onScanProgress, onTreeChanged } from './lib/api'
  import { formatBytes, formatCount } from './lib/format'
  import { getLocale } from './lib/i18n'
  import { initialState, reduce, type UiEvent, type UiState } from './lib/state'
  import { onDarkChange, prefersDark } from './lib/theme'
  import TopBar from './components/TopBar.svelte'
  import StartPage from './components/StartPage.svelte'
  import ScanningPage from './components/ScanningPage.svelte'
  import ResultsPage from './components/ResultsPage.svelte'

  let ui: UiState = $state.raw(initialState)
  let platform = $state('')
  let dark = $state(prefersDark())
  let sameDevice = $state(true)

  const summary = $derived(
    ui.summary
      ? { size: formatBytes(ui.summary.Size, platform), files: formatCount(ui.summary.Files, getLocale()) }
      : null,
  )

  function dispatch(e: UiEvent) {
    ui = reduce(ui, e)
  }

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

  onMount(() => {
    document.documentElement.lang = getLocale() === 'zh' ? 'zh-CN' : 'en'
    const offs = [
      onScanProgress((p) => dispatch({ type: 'progress', progress: p })),
      onScanFinished((e) => dispatch({ type: 'finished', event: e })),
      onTreeChanged((e) => dispatch({ type: 'treeChanged', event: e })),
      onDarkChange((d) => (dark = d)),
    ]
    api.info().then((i) => (platform = i.Platform)).catch(() => {})
    api.state().then(restore).catch(() => {})
    return () => offs.forEach((off) => off())
  })
</script>

<div class="flex h-full flex-col">
  <TopBar phase={ui.phase} root={ui.root} {summary} onBack={() => dispatch({ type: 'reset' })} />
  <main class="min-h-0 flex-1">
    {#if ui.phase === 'start'}
      <StartPage {platform} error={ui.error} bind:sameDevice onScan={startScan} />
    {:else if ui.phase === 'scanning'}
      <ScanningPage root={ui.root} progress={ui.progress} {platform} />
    {:else}
      <ResultsPage
        {ui}
        {platform}
        {dark}
        onNavigate={(id) => dispatch({ type: 'navigate', id })}
        onStale={() => dispatch({ type: 'stale' })}
        onRescan={() => startScan(ui.root)}
      />
    {/if}
  </main>
</div>
