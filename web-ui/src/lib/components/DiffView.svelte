// DiffView — the `mg diff` quick eyeball, upgraded: commits, per-file stat
// bars, and the full patch on demand with proper unified-diff rendering —
// the single most legitimate "the web can do better than the TUI" win.

<script lang="ts">
  import { onMount } from 'svelte'
  import { getJobDiff } from '$lib/api/client'
  import { parseLogOneline, parseStat, parsePatch, type DiffFile } from '$lib/diff'

  let { project, job }: { project: string; job: string } = $props()

  let log = $state('')
  let stat = $state('')
  let patch = $state('')
  let loading = $state(true)
  let loadingFull = $state(false)
  let showFull = $state(false)
  let error = $state('')
  let collapsed = $state<Record<string, boolean>>({})

  const commits = $derived(parseLogOneline(log))
  const statRows = $derived(parseStat(stat))
  const files = $derived(parsePatch(patch))
  const maxBar = $derived(
    Math.max(1, ...statRows.filter((r) => r.file !== '__total__').map((r) => r.adds + r.dels)),
  )

  async function load() {
    loading = true
    try {
      const res = await getJobDiff(project, job)
      log = res.log ?? ''
      stat = res.stat ?? ''
      error = ''
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  async function loadFull() {
    loadingFull = true
    try {
      const res = await getJobDiff(project, job, true)
      patch = res.patch ?? ''
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loadingFull = false
    }
  }

  function toggleFull() {
    showFull = !showFull
    if (showFull && !patch) void loadFull()
  }

  onMount(load)

  function fileKey(f: DiffFile) {
    return f.path
  }
</script>

<div class="diff">
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="none">Reading the branch…</p>
  {:else}
    <section>
      <header class="sec-head">
        <span class="eyebrow">commits</span>
        <button class="btn btn-sm" onclick={toggleFull} aria-expanded={showFull}>
          {showFull ? 'Hide full patch' : 'Full patch'}
        </button>
      </header>
      {#if commits.length === 0}
        <p class="none">No commits on this branch yet.</p>
      {:else}
        <ol class="commits">
          {#each commits as c (c.hash)}
            <li>
              <code class="hash">{c.hash}</code>
              <span class="subject">{c.subject}</span>
            </li>
          {/each}
        </ol>
      {/if}
    </section>

    {#if statRows.length > 0}
      <section>
        <header class="sec-head">
          <span class="eyebrow">changed files</span>
        </header>
        <ul class="stat">
          {#each statRows as r (r.file)}
            {#if r.file === '__total__'}
              <li class="total">
                <span class="file">{r.adds} insertions · {r.dels} deletions</span>
              </li>
            {:else}
              <li>
                <span class="file">{r.file}</span>
                <span class="bar" aria-hidden="true">
                  <span class="adds" style="width: {(r.adds / maxBar) * 100}%"></span>
                  <span class="dels" style="width: {(r.dels / maxBar) * 100}%"></span>
                </span>
                <span class="num">+{r.adds} −{r.dels}</span>
              </li>
            {/if}
          {/each}
        </ul>
      </section>
    {/if}

    {#if showFull}
      <section>
        <header class="sec-head">
          <span class="eyebrow">patch</span>
          {#if loadingFull}<span class="sub">loading…</span>{/if}
        </header>
        {#if files.length === 0 && !loadingFull}
          <p class="none">The patch is empty.</p>
        {/if}
        {#each files as f (fileKey(f))}
          <article class="file">
            <button class="file-head" onclick={() => (collapsed[f.path] = !collapsed[f.path])} aria-expanded={!collapsed[f.path]}>
              <span class="chev" aria-hidden="true">{collapsed[f.path] ? '▸' : '▾'}</span>
              <code>{f.path}</code>
              <span class="file-nums">
                {#if f.adds}<span class="add-n">+{f.adds}</span>{/if}
                {#if f.dels}<span class="del-n">−{f.dels}</span>{/if}
              </span>
            </button>
            {#if !collapsed[f.path]}
              <div class="file-body">
                {#each f.lines as line, i (i)}
                  <div class="line line-{line.kind}">
                    {#if line.kind === 'hunk'}
                      <span class="ln"></span>
                      <span class="code hunk">{line.text}</span>
                    {:else if line.kind === 'meta'}
                      <span class="ln"></span>
                      <span class="code meta">{line.text}</span>
                    {:else}
                      <span class="ln">{line.oldNo ?? ''}</span>
                      <span class="ln">{line.newNo ?? ''}</span>
                      <span class="code" class:plus={line.kind === 'add'} class:minus={line.kind === 'del'}>
                        {line.kind === 'add' ? '+' : line.kind === 'del' ? '−' : ' '}{line.text}
                      </span>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}
          </article>
        {/each}
      </section>
    {/if}
  {/if}
</div>

<style>
  .diff {
    display: flex;
    flex-direction: column;
    gap: var(--r-5);
  }

  .sec-head {
    display: flex;
    align-items: center;
    gap: var(--r-3);
    justify-content: space-between;
    margin-bottom: var(--r-3);
  }
  .sub {
    font-size: 12px;
    color: var(--ink-3);
  }

  .commits {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    background: var(--bg-1);
    overflow: hidden;
  }
  .commits li {
    display: flex;
    gap: var(--r-3);
    align-items: baseline;
    padding: 8px 14px;
    border-bottom: 1px solid var(--line);
    font-size: 13px;
  }
  .commits li:last-child {
    border-bottom: none;
  }
  .hash {
    color: var(--accent-bright);
    font-size: 12px;
    flex: none;
  }
  .subject {
    color: var(--ink-1);
    overflow-wrap: anywhere;
  }

  .stat {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .stat li {
    display: grid;
    grid-template-columns: minmax(140px, 2fr) minmax(80px, 1fr) auto;
    gap: var(--r-3);
    align-items: center;
    font-family: var(--font-mono);
    font-size: 12.5px;
  }
  .stat .file {
    color: var(--ink-1);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .stat .total .file {
    color: var(--ink-2);
  }
  .bar {
    display: flex;
    gap: 1px;
    height: 7px;
    border-radius: 3px;
    overflow: hidden;
  }
  .bar .adds {
    background: var(--st-done);
    border-radius: 2px 0 0 2px;
  }
  .bar .dels {
    background: var(--st-human);
    border-radius: 0 2px 2px 0;
    opacity: 0.85;
  }
  .num {
    color: var(--ink-2);
    font-size: 11.5px;
    white-space: nowrap;
  }

  .file {
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    margin-bottom: var(--r-3);
    overflow: hidden;
    background: var(--bg-1);
  }
  .file-head {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    background: var(--bg-2);
    border: none;
    border-bottom: 1px solid var(--line);
    padding: 7px 12px;
    cursor: pointer;
    font: inherit;
    text-align: left;
  }
  .file-head:hover {
    background: var(--bg-3);
  }
  .file-head code {
    color: var(--ink-0);
    font-size: 12.5px;
    overflow-wrap: anywhere;
  }
  .chev {
    color: var(--ink-2);
    font-size: 11px;
  }
  .file-nums {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: 11.5px;
    display: flex;
    gap: 8px;
  }
  .add-n {
    color: var(--st-done);
  }
  .del-n {
    color: var(--st-human);
  }

  .file-body {
    overflow-x: auto;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.6;
  }
  .line {
    display: flex;
  }
  .line:hover {
    background: rgba(173, 154, 214, 0.04);
  }
  .ln {
    flex: none;
    width: 38px;
    text-align: right;
    padding-right: 8px;
    color: var(--ink-3);
    user-select: none;
    font-size: 10.5px;
    line-height: 1.9;
  }
  .code {
    white-space: pre;
    padding-right: var(--r-4);
    color: var(--ink-1);
  }
  .code.hunk {
    color: var(--st-info);
    background: var(--st-info-dim);
    padding: 0 8px;
  }
  .code.meta {
    color: var(--ink-3);
  }
  .code.plus {
    background: rgba(76, 195, 138, 0.08);
    color: #b4e6cd;
  }
  .code.minus {
    background: rgba(242, 85, 90, 0.08);
    color: #f0b3b5;
  }

  .none {
    color: var(--ink-2);
    font-size: 13.5px;
  }
  .error {
    color: #ffb3b6;
    font-size: 13px;
  }

  @media (max-width: 760px) {
    .stat li {
      grid-template-columns: 1fr auto;
    }
    .stat .bar {
      display: none;
    }
  }
</style>
