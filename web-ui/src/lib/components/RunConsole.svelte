// RunConsole — live run supervision for one job: the run.log event timeline
// (mg-jdi's per-invocation summary) above, the raw session.log stream below.
// The stream attaches over SSE when the daemon offers it (job two) and
// degrades to /jdi polling otherwise — the mode is shown, never hidden.

<script lang="ts">
  import { onMount } from 'svelte'
  import { getJobJdi } from '$lib/api/client'
  import { watchSessionLog, type StreamState } from '$lib/api/stream'
  import { parseRunLog } from '$lib/runlog'

  let { project, job }: { project: string; job: string } = $props()

  let runLog = $state<string | null>(null)
  let stream = $state<StreamState>({ mode: 'idle', text: '' })
  let hasSessionLog = $state(false)
  let error = $state('')
  let follow = $state(true)
  let consoleEl: HTMLElement | undefined = $state()

  const events = $derived(parseRunLog(runLog))
  const humanMarker = $derived(
    [...events].reverse().find((e) => e.kind === 'human')?.text ?? '',
  )

  async function refresh() {
    try {
      const res = await getJobJdi(project, job)
      runLog = res.runLog
      hasSessionLog = res.sessionLog !== null
      if (stream.mode !== 'streaming' && res.sessionLog !== null && !stream.text) {
        stream = { mode: 'polling', text: res.sessionLog }
      }
      error = ''
    } catch (e) {
      if (e instanceof Error && e.name === 'AbortError') return
      error = e instanceof Error ? e.message : String(e)
    }
  }

  onMount(() => {
    void refresh()
    const handle = watchSessionLog(project, job, (s) => (stream = s))
    const poll = setInterval(refresh, 3000)
    return () => {
      handle.stop()
      clearInterval(poll)
    }
  })

  // follow the tail unless the user scrolled up
  $effect(() => {
    if (follow && consoleEl && stream.text) {
      consoleEl.scrollTop = consoleEl.scrollHeight
    }
  })

  function onScroll() {
    if (!consoleEl) return
    follow = consoleEl.scrollHeight - consoleEl.scrollTop - consoleEl.clientHeight < 40
  }
</script>

<div class="run">
  <section class="timeline" aria-label="Run timeline">
    <header class="sec-head">
      <span class="eyebrow">run.log</span>
      <span class="sub">invocation events</span>
    </header>
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    {#if events.length === 0}
      <p class="none">mg jdi has not driven this job yet.</p>
    {:else}
      <ol>
        {#each events as ev, i (i)}
          <li
            class="ev ev-{ev.kind}"
            title={ev.timestamp ?? ''}
          >
            {#if ev.kind === 'invoke'}
              <span class="who">@{ev.agent}</span>
              <span class="what">invoked</span>
              <span class="att">{ev.text.match(/attempt \d+/)?.[0] ?? ''}</span>
            {:else if ev.kind === 'start'}
              <span class="who">mg jdi</span>
              <span class="what">{ev.text}</span>
            {:else if ev.kind === 'human'}
              <span class="who">handoff</span>
              <span class="what human-text">NEEDS-HUMAN-INPUT: {ev.text}</span>
            {:else if ev.kind === 'stop'}
              <span class="who">stopped</span>
              <span class="what" class:stopped-human={ev.text.includes('needs human')}>{ev.text}</span>
            {:else}
              <span class="who">{ev.agent ? '@' + ev.agent : '·'}</span>
              <span class="what">{ev.text}</span>
            {/if}
          </li>
        {/each}
      </ol>
    {/if}
  </section>

  <section class="console-sec" aria-label="Live session output">
    <header class="sec-head">
      <span class="eyebrow">session.log</span>
      <span class="live-pill" class:live={stream.mode === 'streaming'} class:polling={stream.mode === 'polling'}>
        {stream.mode === 'streaming' ? 'streaming' : stream.mode === 'polling' ? 'polling' : 'idle'}
      </span>
    </header>
    {#if !hasSessionLog && !stream.text}
      <p class="none">No captured run output yet — the first detached run creates session.log.</p>
    {:else}
      <div class="console" bind:this={consoleEl} onscroll={onScroll} tabindex="0" role="log" aria-label="Agent session output">
        <pre>{stream.text || '(empty)'}</pre>
      </div>
      {#if !follow}
        <button class="btn btn-sm follow" onclick={() => (follow = true)}>↓ Follow</button>
      {/if}
    {/if}
  </section>
</div>

<style>
  .run {
    display: flex;
    flex-direction: column;
    gap: var(--r-5);
  }

  .sec-head {
    display: flex;
    align-items: baseline;
    gap: var(--r-3);
    margin-bottom: var(--r-3);
  }
  .sub {
    font-size: 12px;
    color: var(--ink-3);
  }

  .live-pill {
    font-family: var(--font-mono);
    font-size: 10.5px;
    letter-spacing: 0.1em;
    padding: 1px 8px;
    border-radius: 99px;
    border: 1px solid var(--line-strong);
    color: var(--ink-2);
    text-transform: uppercase;
  }
  .live-pill.live {
    color: var(--accent-bright);
    border-color: rgba(139, 108, 246, 0.5);
    background: var(--st-running-dim);
  }
  .live-pill.live::before {
    content: '';
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent-bright);
    margin-right: 6px;
    animation: blink 1.4s ease-in-out infinite;
    vertical-align: 1px;
  }
  .live-pill.polling {
    color: var(--st-open);
    border-color: rgba(227, 179, 65, 0.4);
  }
  @keyframes blink {
    50% {
      opacity: 0.35;
    }
  }

  .timeline ol {
    list-style: none;
    margin: 0;
    padding: 0;
    border-left: 1px solid var(--line-strong);
    margin-left: 5px;
  }
  .ev {
    position: relative;
    display: flex;
    gap: 10px;
    align-items: baseline;
    padding: 5px 0 5px 18px;
    font-size: 13.5px;
    flex-wrap: wrap;
  }
  .ev::before {
    content: '';
    position: absolute;
    left: -4px;
    top: 11px;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--bg-1);
    border: 1.5px solid var(--ink-3);
  }
  .ev-invoke::before {
    border-color: var(--accent);
    background: var(--accent);
  }
  .ev-start::before {
    border-color: var(--st-info);
  }
  .ev-human::before {
    border-color: var(--st-human);
    background: var(--st-human);
  }
  .ev-stop::before {
    border-color: var(--ink-2);
  }

  .who {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-2);
    min-width: 92px;
    flex: none;
  }
  .ev-invoke .who {
    color: var(--accent-bright);
  }
  .ev-human .who {
    color: #ff8d91;
  }
  .what {
    color: var(--ink-1);
  }
  .att {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--ink-3);
  }
  .human-text {
    color: #ffb3b6;
    font-family: var(--font-mono);
    font-size: 12.5px;
  }
  .stopped-human {
    color: #ffb3b6;
  }

  .console {
    background: #100d16;
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    padding: var(--r-3) var(--r-4);
    max-height: 420px;
    overflow-y: auto;
    position: relative;
  }
  .console pre {
    margin: 0;
    font-size: 12.5px;
    line-height: 1.65;
    color: var(--ink-1);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .follow {
    margin-top: var(--r-2);
  }

  .none {
    color: var(--ink-2);
    font-size: 13.5px;
    padding: var(--r-3) 0;
  }
  .error {
    color: #ffb3b6;
    font-size: 13px;
  }

  @media (max-width: 760px) {
    .who {
      min-width: 0;
    }
  }
</style>
