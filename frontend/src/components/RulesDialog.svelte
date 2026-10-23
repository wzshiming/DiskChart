<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { api } from '../lib/api'
  import { t } from '../lib/i18n'
  import { getPlatform } from '../lib/prefs.svelte'
  import { appendUnique, rulesEqual, validateName, validatePath, validateRecommendedPath } from '../lib/rules'
  import Button from './ui/Button.svelte'
  import Icon from './ui/Icon.svelte'

  interface Props {
    open: boolean
    onClose: () => void
  }
  let { open, onClose }: Props = $props()

  type Kind = 'Protected' | 'Recommended' | 'Names'
  type Lists = Record<Kind, string[]>

  const sections: { kind: Kind; title: string; hint: string; placeholder: string; path: boolean }[] = [
    {
      kind: 'Protected',
      title: 'rules.protected',
      hint: 'rules.protectedHint',
      placeholder: 'rules.pathPlaceholder',
      path: true,
    },
    {
      kind: 'Recommended',
      title: 'rules.recommended',
      hint: 'rules.recommendedHint',
      placeholder: 'rules.pathPlaceholder',
      path: true,
    },
    { kind: 'Names', title: 'rules.names', hint: 'rules.namesHint', placeholder: 'rules.namePlaceholder', path: false },
  ]

  const emptyLists = (): Lists => ({ Protected: [], Recommended: [], Names: [] })
  const emptyDrafts = (): Record<Kind, string> => ({ Protected: '', Recommended: '', Names: '' })

  let dlg = $state<HTMLDialogElement>()
  let info = $state.raw<app.UserRulesInfo | null>(null)
  let loading = $state(false)
  let loadError = $state('')
  let lists = $state.raw<Lists>(emptyLists())
  let drafts = $state(emptyDrafts())
  let saving = $state(false)
  let saveError = $state('')
  let pickError = $state('')
  let armed = $state(false)
  let seq = 0

  function validate(kind: Kind, v: string): string {
    switch (kind) {
      case 'Protected':
        return validatePath(v, getPlatform())
      case 'Recommended':
        return validateRecommendedPath(v, getPlatform())
      case 'Names':
        return validateName(v)
    }
  }

  function canAdd(kind: Kind): boolean {
    const v = drafts[kind]
    return v.trim() !== '' && validate(kind, v) === ''
  }

  const allValid = $derived(sections.every((s) => lists[s.kind].every((v) => validate(s.kind, v) === '')))
  const allEmpty = $derived(sections.every((s) => lists[s.kind].length === 0))
  const canSave = $derived(
    info !== null && !saving && allValid && (!!info.LoadError || !rulesEqual(lists, info.Rules ?? {})),
  )

  function reset() {
    seq++
    info = null
    loading = false
    loadError = ''
    lists = emptyLists()
    drafts = emptyDrafts()
    saving = false
    saveError = ''
    pickError = ''
    armed = false
  }

  async function load() {
    const n = ++seq
    loading = true
    loadError = ''
    try {
      const i = await api.userRules()
      if (n !== seq) return
      info = i
      lists = {
        Protected: [...(i.Rules?.Protected ?? [])],
        Recommended: [...(i.Rules?.Recommended ?? [])],
        Names: [...(i.Rules?.Names ?? [])],
      }
    } catch (e) {
      if (n !== seq) return
      loadError = String(e)
    } finally {
      if (n === seq) loading = false
    }
  }

  $effect(() => {
    if (open && dlg && !dlg.open) {
      reset()
      dlg.showModal()
      load()
    } else if (!open && dlg?.open) {
      dlg.close()
    }
  })

  function setList(kind: Kind, next: string[]) {
    if (next === lists[kind]) return
    lists = { ...lists, [kind]: next }
    armed = false
  }

  function add(kind: Kind) {
    if (saving || !canAdd(kind)) return
    setList(kind, appendUnique(lists[kind], drafts[kind]))
    drafts[kind] = ''
  }

  function remove(kind: Kind, index: number) {
    if (saving) return
    setList(
      kind,
      lists[kind].filter((_, i) => i !== index),
    )
  }

  async function pick(kind: Kind) {
    pickError = ''
    try {
      const dir = await api.pickDirectory(t('rules.pickFolderTitle'))
      if (dir && !saving) setList(kind, appendUnique(lists[kind], dir))
    } catch (e) {
      pickError = String(e)
    }
  }

  function clearAll() {
    if (!armed) {
      armed = true
      return
    }
    lists = emptyLists()
    armed = false
  }

  async function save() {
    if (!canSave) return
    saving = true
    saveError = ''
    try {
      await api.setUserRules({ Protected: lists.Protected, Recommended: lists.Recommended, Names: lists.Names })
      dlg?.close()
    } catch (e) {
      saveError = String(e)
    } finally {
      saving = false
    }
  }

  const input =
    'h-7 w-full rounded-lg border border-zinc-300 bg-white px-2.5 font-mono text-xs text-zinc-800 shadow-sm placeholder:font-sans placeholder:text-zinc-400 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500'
  const errorText = 'text-xs text-red-600 dark:text-red-400'
</script>

<dialog
  bind:this={dlg}
  aria-labelledby="rules-dialog-title"
  class="m-auto w-[min(640px,92vw)] rounded-2xl border border-zinc-200 bg-white p-6 text-zinc-900 shadow-2xl backdrop:bg-black/40 backdrop:backdrop-blur-[2px] dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
  onclose={onClose}
  oncancel={(e) => saving && e.preventDefault()}
>
  {#if open}
    <div class="flex items-center gap-3">
      <span
        class="flex size-9 shrink-0 items-center justify-center rounded-full bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
      >
        <Icon name="cog" class="size-5" />
      </span>
      <h2 id="rules-dialog-title" class="text-base font-semibold">{t('rules.title')}</h2>
    </div>
    <p class="mt-3 text-sm text-zinc-600 dark:text-zinc-300">{t('rules.intro')}</p>

    {#if loading}
      <p class="mt-4 text-sm text-zinc-500">{t('common.loading')}</p>
    {:else if loadError}
      <div class="mt-4 rounded-md bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-950/50 dark:text-red-300">
        <span class="font-medium">{t('rules.loadFailed')}</span>: <span class="break-all">{loadError}</span>
        <div class="mt-2">
          <Button size="sm" icon="arrow-path" onclick={load}>{t('common.retry')}</Button>
        </div>
      </div>
    {:else if info}
      {#if info.LoadError}
        <p
          class="mt-4 rounded-md bg-amber-50 px-3 py-2 text-xs break-all text-amber-800 dark:bg-amber-950/40 dark:text-amber-300"
        >
          {t('rules.loadError', { error: info.LoadError })}
        </p>
      {/if}

      <div class="mt-4 max-h-[55vh] space-y-5 overflow-y-auto pr-1">
        {#each sections as s (s.kind)}
          {@const items = lists[s.kind]}
          {@const draftError = validate(s.kind, drafts[s.kind])}
          <section>
            <h3 class="text-sm font-medium">{t(s.title)}</h3>
            <p class="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">{t(s.hint)}</p>
            {#if items.length === 0}
              <p class="mt-2 text-xs text-zinc-400 dark:text-zinc-500">{t('rules.empty')}</p>
            {:else}
              <ul
                class="mt-2 divide-y divide-zinc-100 rounded-lg border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-800"
              >
                {#each items as v, i}
                  {@const err = validate(s.kind, v)}
                  <li class="flex items-center gap-2 px-3 py-1">
                    <span class="min-w-0 flex-1">
                      <span class="block font-mono text-xs break-all {err ? 'text-red-600 dark:text-red-400' : ''}">
                        {v}
                      </span>
                      {#if err}<span class="block {errorText}">{t(err)}</span>{/if}
                    </span>
                    <Button
                      variant="ghost"
                      size="sm"
                      icon="x-mark"
                      title={t('rules.remove')}
                      ariaLabel={t('rules.remove')}
                      disabled={saving}
                      onclick={() => remove(s.kind, i)}
                    />
                  </li>
                {/each}
              </ul>
            {/if}
            <form
              class="mt-2 flex items-start gap-2"
              onsubmit={(e) => {
                e.preventDefault()
                add(s.kind)
              }}
            >
              <div class="min-w-0 flex-1">
                <input
                  type="text"
                  class={input}
                  placeholder={t(s.placeholder)}
                  aria-label={t(s.title)}
                  autocomplete="off"
                  spellcheck="false"
                  disabled={saving}
                  bind:value={drafts[s.kind]}
                />
                {#if draftError}<p class="mt-1 {errorText}">{t(draftError)}</p>{/if}
              </div>
              {#if s.path}
                <Button size="sm" icon="folder-open" disabled={saving} onclick={() => pick(s.kind)}>
                  {t('rules.pickFolder')}
                </Button>
              {/if}
              <Button type="submit" size="sm" disabled={saving || !canAdd(s.kind)}>{t('rules.add')}</Button>
            </form>
          </section>
        {/each}
      </div>

      {#if pickError}
        <p class="mt-3 break-all {errorText}">{t('common.error')}: {pickError}</p>
      {/if}
      {#if saveError}
        <div class="mt-3 rounded-md bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-950/50 dark:text-red-300">
          <span class="font-medium">{t('rules.saveFailed')}</span>
          <pre class="mt-1 font-mono break-all whitespace-pre-wrap">{saveError}</pre>
        </div>
      {/if}
      {#if info.Path}
        <p class="mt-3 text-xs break-all text-zinc-500 dark:text-zinc-400">{t('rules.storedAt', { path: info.Path })}</p>
      {/if}
    {/if}

    <div class="mt-4 flex items-center justify-between gap-3">
      <div>
        {#if info}
          <Button
            variant={armed ? 'danger' : 'danger-outline'}
            size="sm"
            disabled={saving || allEmpty}
            onclick={clearAll}
          >
            {t(armed ? 'rules.clearAllConfirm' : 'rules.clearAll')}
          </Button>
        {/if}
      </div>
      <div class="flex shrink-0 gap-2">
        <Button disabled={saving} onclick={() => dlg?.close()}>{t('rules.cancel')}</Button>
        <Button variant="primary" icon={saving ? 'spinner' : undefined} disabled={!canSave} onclick={save}>
          {t(saving ? 'rules.saving' : 'rules.save')}
        </Button>
      </div>
    </div>
  {/if}
</dialog>
