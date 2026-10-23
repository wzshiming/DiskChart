export interface Refresher {
  // Runs at once and drops any pending later().
  now(): void
  // Trailing debounce: runs once, delayMs after the last call.
  later(): void
  cancel(): void
}

export function createRefresher(run: () => void, delayMs = 200): Refresher {
  let timer: ReturnType<typeof setTimeout> | undefined

  function cancel() {
    if (timer === undefined) return
    clearTimeout(timer)
    timer = undefined
  }

  return {
    now() {
      cancel()
      run()
    },
    later() {
      cancel()
      timer = setTimeout(() => {
        timer = undefined
        run()
      }, delayMs)
    },
    cancel,
  }
}
