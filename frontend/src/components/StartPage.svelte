<script lang="ts">
  import { onMount } from 'svelte'
  import type { app } from '../../wailsjs/go/models'
  import { api } from '../lib/api'
  import { t } from '../lib/i18n'
  import Button from './ui/Button.svelte'
  import Icon from './ui/Icon.svelte'
  import VolumeCard from './VolumeCard.svelte'

  interface Props {
    error: string
    sameDevice: boolean
    onScan: (root: string) => void
  }
  let { error, sameDevice = $bindable(), onScan }: Props = $props()

  let volumes = $state.raw<app.Volume[] | null>(null)
  let volumesError = $state('')
  let pickError = $state('')

  function loadVolumes() {
    volumes = null
    volumesError = ''
    api
      .listVolumes()
      .then((v) => (volumes = v ?? []))
      .catch((e) => {
        volumes = []
        volumesError = String(e)
      })
  }

  onMount(loadVolumes)

  async function pick() {
    try {
      const dir = await api.pickDirectory(t('start.pickFolderTitle'))
      if (dir) onScan(dir)
    } catch (e) {
      pickError = String(e)
    }
  }

  const shownError = $derived(error || pickError)
  const card = 'rounded-2xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900'
  const focus = 'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500'
</script>

<div class="h-full overflow-y-auto">
  <div class="mx-auto flex max-w-5xl flex-col gap-10 px-8 py-10">
    {#if shownError}
      <div
        class="flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200"
        role="alert"
      >
        <Icon name="exclamation-triangle" class="mt-0.5 size-4" />
        <div class="min-w-0">
          <span class="font-medium">{t('common.error')}</span>
          <span class="ml-2 break-all">{shownError}</span>
        </div>
      </div>
    {/if}

    <section class="flex flex-col gap-4">
      <div class="flex items-baseline justify-between gap-4">
        <h2 class="text-sm font-semibold tracking-wide text-zinc-500 uppercase dark:text-zinc-400">
          {t('start.volumes')}
        </h2>
        {#if volumes && volumes.length > 0}
          <span class="text-xs text-zinc-400 tabular-nums dark:text-zinc-500">
            {volumes.length === 1 ? t('start.volumeCountOne') : t('start.volumeCount', { count: volumes.length })}
          </span>
        {/if}
      </div>

      <div class="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-4">
        {#if volumes === null}
          {#each [0, 1, 2] as i (i)}
            <div class="{card} flex animate-pulse flex-col gap-4 p-5 shadow-sm" aria-hidden="true">
              <div class="flex items-center gap-3">
                <span class="size-10 rounded-xl bg-zinc-200 dark:bg-zinc-800"></span>
                <div class="flex flex-1 flex-col gap-2">
                  <span class="h-3.5 w-24 rounded bg-zinc-200 dark:bg-zinc-800"></span>
                  <span class="h-3 w-16 rounded bg-zinc-200 dark:bg-zinc-800"></span>
                </div>
              </div>
              <span class="h-2 rounded-full bg-zinc-200 dark:bg-zinc-800"></span>
              <div class="flex flex-col gap-2">
                <span class="h-3.5 w-40 rounded bg-zinc-200 dark:bg-zinc-800"></span>
                <span class="h-3 w-20 rounded bg-zinc-200 dark:bg-zinc-800"></span>
              </div>
            </div>
          {/each}
          <span class="sr-only">{t('common.loading')}</span>
        {:else if volumesError}
          <div class="{card} col-span-full flex items-center gap-4 p-5">
            <span
              class="flex size-10 shrink-0 items-center justify-center rounded-xl bg-red-50 text-red-600 dark:bg-red-950/40 dark:text-red-300"
            >
              <Icon name="exclamation-triangle" class="size-5" />
            </span>
            <div class="min-w-0 flex-1 text-sm">
              <div class="font-medium">{t('common.error')}</div>
              <div class="break-all text-zinc-500 dark:text-zinc-400">{volumesError}</div>
            </div>
            <Button size="sm" icon="arrow-path" onclick={loadVolumes}>{t('common.retry')}</Button>
          </div>
        {:else if volumes.length === 0}
          <div class="{card} col-span-full flex items-center gap-4 p-5">
            <span
              class="flex size-10 shrink-0 items-center justify-center rounded-xl bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400"
            >
              <Icon name="hard-drive" class="size-5" />
            </span>
            <p class="text-sm text-zinc-600 dark:text-zinc-300">{t('start.noVolumes')}</p>
          </div>
        {:else}
          {#each volumes as v (v.Mountpoint)}
            <VolumeCard volume={v} onclick={() => onScan(v.Mountpoint)} />
          {/each}
        {/if}

        <button
          type="button"
          class="group flex min-h-[150px] flex-col items-start justify-center gap-2 rounded-2xl border-2 border-dashed border-zinc-300 p-5 text-left transition hover:border-blue-400 hover:bg-blue-50/40 dark:border-zinc-700 dark:hover:border-blue-500 dark:hover:bg-blue-950/20 {focus}"
          onclick={pick}
        >
          <span
            class="flex size-10 items-center justify-center rounded-xl bg-blue-50 text-blue-600 transition group-hover:scale-105 dark:bg-blue-950/40 dark:text-blue-300"
          >
            <Icon name="folder-open" class="size-5" />
          </span>
          <span class="font-semibold">{t('start.pickFolder')}</span>
          <span class="text-xs text-zinc-500 dark:text-zinc-400">{t('start.pickFolderHint')}</span>
        </button>
      </div>
    </section>

    <section class="{card} flex items-center justify-between gap-6 p-4">
      <label for="same-device" class="min-w-0 cursor-pointer select-none">
        <span id="same-device-label" class="block text-sm font-semibold">{t('start.sameDevice')}</span>
        <span id="same-device-hint" class="mt-0.5 block text-xs text-zinc-500 dark:text-zinc-400">
          {t('start.sameDeviceHint')}
        </span>
      </label>
      <button
        id="same-device"
        type="button"
        role="switch"
        aria-checked={sameDevice}
        aria-labelledby="same-device-label"
        aria-describedby="same-device-hint"
        class="relative h-6 w-11 shrink-0 rounded-full transition-colors {focus} {sameDevice
          ? 'bg-blue-600 dark:bg-blue-500'
          : 'bg-zinc-300 dark:bg-zinc-700'}"
        onclick={() => (sameDevice = !sameDevice)}
      >
        <span
          class="absolute top-0.5 left-0 size-5 rounded-full bg-white shadow transition-transform {sameDevice
            ? 'translate-x-5.5'
            : 'translate-x-0.5'}"
        ></span>
      </button>
    </section>
  </div>
</div>
