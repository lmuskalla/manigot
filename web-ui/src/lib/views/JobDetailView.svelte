<script lang="ts">
  import { onMount } from 'svelte'
  import Pipeline from '$lib/components/Pipeline.svelte'
  import MarkdownView from '$lib/components/MarkdownView.svelte'
  import Modal from '$lib/components/Modal.svelte'
  import Confirm from '$lib/components/Confirm.svelte'
  import RunConsole from '$lib/components/RunConsole.svelte'
  import DiffView from '$lib/components/DiffView.svelte'
  import { data } from '$lib/state/data.svelte'
  import { toasts } from '$lib/state/toasts.svelte'
  import { navigate, href } from '$lib/router'
  import { relativeTime } from '$lib/time'
  import { verdictStatus } from '$lib/markdown'
  import * as api from '$lib/api/client'
  import type { AgentRow, JobRow } from '$lib/api/types'

  let { project, jobName, tab = 'brief' }: { project: string; jobName: string; tab?: string } = $props()

  const TABS = [
    { id: 'brief', label: 'Brief' },
    { id: 'tasks', label: 'Tasks' },
    { id: 'implementation', label: 'Implementation' },
    { id: 'verdict', label: 'Verdict' },
    { id: 'diff', label: 'Diff' },
    { id: 'run', label: 'Run' },
  ] as const

  const job = $derived<JobRow | undefined>(
    (data.active === project ? data.jobs : []).find(
      (j) => j.name === jobName || j.id === jobName || j.name.startsWith(jobName + '_'),
    ),
  )

  // file content per tab
  let fileContent = $state('')
  let fileLoading = $state(false)
  let fileMissing = $state(false)

  // brief editing (Part 2)
  let editing = $state(false)
  let draft = $state('')
  let saving = $state(false)

  // agents + launches (Part 2)
  let agents = $state<AgentRow[]>([])
  let showAgents = $state(false)
  let agentQuery = $state('')
  let launchingAgent = $state('')
  let profile = $state('')

  // jdi run
  let showJdi = $state(false)
  let startingJdi = $state(false)

  // lifecycle confirms
  let confirmKind = $state<null | 'done' | 'delete' | 'push'>(null)
  let lifecycleBusy = $state(false)

  let verdict = $state('')
  let verdictApproved = $state(false)

  $effect(() => {
    // (re)load whenever the tab or job changes
    jobName
    void loadTab()
  })

  onMount(async () => {
    if (data.active !== project) {
      data.setActive(project)
      await data.refreshJobs()
    } else if (!job) {
      await data.refreshJobs() // direct link — the list may not know this job yet
    }
    void loadAgents()
    data.startPolling()
    return () => data.stopPolling()
  })

  async function loadTab() {
    if (tab === 'diff' || tab === 'run') return
    fileLoading = true
    fileMissing = false
    try {
      fileContent = await api.getJobFile(project, jobName, tab)
      if (tab === 'verdict') {
        verdict = verdictStatus(fileContent)
        verdictApproved = /\bAPPROVED\b/i.test(verdict) && !/REJECTED|NEEDS WORK/i.test(verdict)
      }
    } catch {
      fileContent = ''
      fileMissing = true
    } finally {
      fileLoading = false
    }
  }

  async function loadAgents() {
    try {
      const res = await api.getAgents(project)
      agents = res.agents ?? []
    } catch {
      agents = []
    }
  }

  function goTab(t: string) {
    navigate(href({ name: 'job', project, job: jobName, tab: t }))
  }

  function actionError(e: unknown, what: string): string {
    if (e instanceof api.ApiError && e.capabilityMiss) {
      return `This daemon does not expose ${what} yet — mutating endpoints land with job two of the control plane.`
    }
    return e instanceof Error ? e.message : String(e)
  }

  async function startEdit() {
    draft = fileContent
    editing = true
  }

  async function saveBrief() {
    saving = true
    try {
      await api.saveBrief(project, jobName, draft)
      toasts.ok('Brief saved')
      editing = false
      fileContent = draft
      await data.refreshJobs()
    } catch (e) {
      toasts.error(actionError(e, 'brief editing'))
    } finally {
      saving = false
    }
  }

  async function launch(agent: string) {
    launchingAgent = agent
    try {
      const res = await api.launchAgent(project, jobName, agent, profile || undefined)
      toasts.ok(res.message ?? `@${agent} launched detached`)
      showAgents = false
      await data.refreshJobs()
    } catch (e) {
      toasts.error(actionError(e, 'agent launches'))
    } finally {
      launchingAgent = ''
    }
  }

  async function runJdi() {
    startingJdi = true
    try {
      const res = await api.startJdi(project, jobName, profile || undefined)
      toasts.ok(res.message ?? 'mg jdi detached — watch the Run tab')
      showJdi = false
      goTab('run')
      await data.refreshJobs()
    } catch (e) {
      toasts.error(actionError(e, 'jdi launches'))
    } finally {
      startingJdi = false
    }
  }

  async function lifecycle(kind: 'done' | 'delete' | 'push') {
    lifecycleBusy = true
    try {
      if (kind === 'done') {
        const res = await api.doneJob(project, jobName)
        toasts.ok(res.message ?? 'Job archived and squash-merged')
      } else if (kind === 'delete') {
        const res = await api.deleteJob(project, jobName)
        toasts.ok(res.message ?? 'Job deleted')
      } else {
        const res = await api.pushJob(project, jobName)
        toasts.ok(res.message ?? 'Branch pushed')
      }
      confirmKind = null
      if (kind === 'delete') navigate(href({ name: 'jobs', project }))
      else await data.refreshJobs()
    } catch (e) {
      toasts.error(actionError(e, kind === 'done' ? 'done' : kind === 'delete' ? 'delete' : 'push'))
    } finally {
      lifecycleBusy = false
    }
  }

  const filteredAgents = $derived.by(() => {
    const q = agentQuery.trim().toLowerCase()
    if (!q) return agents
    return agents.filter(
      (a) => a.name.toLowerCase().includes(q) || a.description.toLowerCase().includes(q),
    )
  })

  const needsHuman = $derived(job?.jdi?.state === 'stopped:needs-human')
  const isRunning = $derived(job?.jdi?.state === 'running')
</script>

<div class="detail">
  <header class="head">
    <a class="back" href={href({ name: 'jobs', project })} aria-label="Back to jobs">←</a>
    <div class="titles">
      <h1>{job?.title ?? jobName}</h1>
      <p class="ident">
        <span class="mono">{job?.name ?? jobName}</span>
        {#if job?.branch}<span class="sep">·</span><span class="mono dim">{job.branch}</span>{/if}
        {#if job}<span class="sep">·</span><span>{job.type}</span>{/if}
        {#if job?.date}<span class="sep">·</span><span class="dim">{job.date}</span>{/if}
        {#if job?.jdi}
          <span class="sep">·</span>
          <span class="dim">{job.jdi.agent} · {relativeTime(job.jdi.updated)}</span>
        {/if}
      </p>
    </div>
    <div class="actions">
      <button class="btn" class:btn-live={isRunning} onclick={() => (showJdi = true)}>
        {#if isRunning}mg jdi running…{:else}Run mg jdi{/if}
      </button>
      <button class="btn" onclick={() => (showAgents = true)}>Launch agent</button>
      <div class="more" role="group" aria-label="Lifecycle">
        <button class="btn btn-sm" onclick={() => (confirmKind = 'push')} disabled={!job?.branch}>
          Push
        </button>
        <button
          class="btn btn-sm"
          class:btn-primary={job?.stage === 'finished'}
          onclick={() => (confirmKind = 'done')}
        >
          Done
        </button>
        <button class="btn btn-sm btn-danger" onclick={() => (confirmKind = 'delete')}>Delete</button>
      </div>
    </div>
  </header>

  {#if job}
    <div class="stage-row">
      <Pipeline stage={job.stage} jdi={job.jdi} variant="full" />
      {#if job.jdi?.state === 'running'}
        <span class="chip chip-running"><span class="dot"></span> {job.jdi.agent} running</span>
      {:else if needsHuman}
        <span class="chip chip-human"><span class="dot"></span> stopped for a human</span>
      {/if}
    </div>
  {/if}

  {#if needsHuman}
    <aside class="human" role="alert">
      <div class="human-body">
        <p class="eyebrow human-eyebrow">handoff</p>
        <p class="human-text">
          The run stopped with <code>NEEDS-HUMAN-INPUT</code> — a decision only you can make.
          Answer it in the brief (or the relevant job file), then relaunch the sequence.
        </p>
      </div>
      <div class="human-actions">
        <button class="btn" onclick={() => (tab === 'brief' ? startEdit() : goTab('brief'))}>
          Edit brief
        </button>
        <button class="btn btn-primary" onclick={() => (showJdi = true)}>Relaunch mg jdi</button>
      </div>
    </aside>
  {/if}

  <nav class="tabs" role="tablist" aria-label="Job files">
    {#each TABS as t (t.id)}
      <button role="tab" aria-selected={tab === t.id} class:active={tab === t.id} onclick={() => goTab(t.id)}>
        {t.label}
        {#if t.id === 'verdict' && verdict && tab !== 'verdict'}
          <span class="v-dot" class:approved={verdictApproved}></span>
        {/if}
      </button>
    {/each}
  </nav>

  <div class="body" role="tabpanel">
    {#if tab === 'diff'}
      <DiffView {project} job={jobName} />
    {:else if tab === 'run'}
      <RunConsole {project} job={jobName} />
    {:else if editing && tab === 'brief'}
      <div class="editor">
        <textarea
          class="textarea"
          bind:value={draft}
          spellcheck="false"
          aria-label="Editing brief.md"
        ></textarea>
        <div class="editor-bar">
          <span class="hint mono">brief.md — raw markdown replaces the file</span>
          <div class="editor-actions">
            <button class="btn" onclick={() => (editing = false)} disabled={saving}>Cancel</button>
            <button class="btn btn-primary" onclick={saveBrief} disabled={saving}>
              {#if saving}<span class="spin" aria-hidden="true"></span>{/if}
              Save brief
            </button>
          </div>
        </div>
      </div>
    {:else}
      {#if fileLoading}
        <p class="loading">Reading {tab}.md…</p>
      {:else if fileMissing}
        <div class="missing">
          <p class="mono">no {tab}.md yet</p>
          <p class="dim">
            {#if tab === 'tasks'}The analyst writes this (@analyst, or Run mg jdi).
            {:else if tab === 'implementation'}The developer writes this as tasks complete.
            {:else if tab === 'verdict'}The reviewer writes this after review.
            {:else}The brief is the job's first file.{/if}
          </p>
        </div>
      {:else}
        <MarkdownView source={fileContent} />
        {#if tab === 'brief'}
          <div class="file-actions">
            <button class="btn" onclick={startEdit}>Edit brief</button>
          </div>
        {/if}
        {#if tab === 'verdict' && verdict}
          <p class="verdict-line" class:approved={verdictApproved} class:rejected={!verdictApproved}>
            Verdict: {verdict}
          </p>
        {/if}
      {/if}
    {/if}
  </div>
</div>

{#if showAgents}
  <Modal title="Launch agent" onclose={() => (showAgents = false)} width="560px">
    <p class="launch-hint">
      One detached run in the job's worktree — its output lands in <code>session.log</code> and the
      Run tab. Interactive sessions stay in the terminal by design.
    </p>
    <div class="field">
      <label for="launch-profile">profile</label>
      <select id="launch-profile" class="select" bind:value={profile}>
        <option value="">daemon default</option>
        <option value="claude-pro">claude-pro</option>
        <option value="zai">zai</option>
        <option value="opencode-go">opencode-go</option>
        <option value="opencode-zen">opencode-zen</option>
        <option value="opencode-zen-free">opencode-zen-free</option>
      </select>
    </div>
    <input class="input" placeholder="Filter the crew…" bind:value={agentQuery} spellcheck="false" aria-label="Filter agents" />
    <ul class="crew">
      {#each filteredAgents as a (a.name)}
        <li>
          <button class="crew-row" onclick={() => launch(a.name)} disabled={launchingAgent !== ''}>
            <span class="crew-name">{"@" + a.name}</span>
            <span class="crew-desc">{a.description}</span>
            {#if launchingAgent === a.name}<span class="spin" aria-hidden="true"></span>{/if}
          </button>
        </li>
      {/each}
    </ul>
    {#snippet footer()}
      <button class="btn" onclick={() => (showAgents = false)} disabled={launchingAgent !== ''}>Cancel</button>
    {/snippet}
  </Modal>
{/if}

{#if showJdi}
  <Modal title="Run mg jdi" busy={startingJdi} onclose={() => (showJdi = false)} width="460px">
    <p class="launch-hint">
      The fixed sequence — <code>@analyst → @developer → @reviewer</code> — runs detached, end to
      end. It stops on an approved verdict, a human decision, or its retry budget.
    </p>
    <div class="field">
      <label for="jdi-profile">profile</label>
      <select id="jdi-profile" class="select" bind:value={profile}>
        <option value="">daemon default (claude-pro)</option>
        <option value="claude-pro">claude-pro</option>
        <option value="zai">zai</option>
        <option value="opencode-go">opencode-go</option>
        <option value="opencode-zen">opencode-zen</option>
        <option value="opencode-zen-free">opencode-zen-free</option>
      </select>
    </div>
    {#snippet footer()}
      <button class="btn" onclick={() => (showJdi = false)} disabled={startingJdi}>Cancel</button>
      <button class="btn btn-primary" onclick={runJdi} disabled={startingJdi}>
        {#if startingJdi}<span class="spin" aria-hidden="true"></span>{/if}
        Launch sequence
      </button>
    {/snippet}
  </Modal>
{/if}

{#if confirmKind === 'done'}
  <Confirm
    title="Archive and merge"
    body={job?.stage === 'finished'
      ? `Done squash-merges ${jobName} into the base branch and archives its docs.`
      : `The verdict is not approved yet (${verdict || 'no verdict status found'}). mg done can still archive and merge — continue?`}
    confirmLabel="Archive & merge"
    confirmKind="primary"
    busy={lifecycleBusy}
    onconfirm={() => lifecycle('done')}
    onclose={() => (confirmKind = null)}
  />
{:else if confirmKind === 'delete'}
  <Confirm
    title="Delete job"
    body={`Permanently delete ${jobName}?\nThe worktree and branch are removed — uncommitted changes will be discarded.\n\nThis cannot be undone.`}
    confirmLabel="Delete permanently"
    confirmKind="danger"
    busy={lifecycleBusy}
    onconfirm={() => lifecycle('delete')}
    onclose={() => (confirmKind = null)}
  />
{:else if confirmKind === 'push'}
  <Confirm
    title="Push branch"
    body={`Push ${job?.branch ?? jobName} to origin?`}
    confirmLabel="Push"
    busy={lifecycleBusy}
    onconfirm={() => lifecycle('push')}
    onclose={() => (confirmKind = null)}
  />
{/if}

<style>
  .detail {
    max-width: 880px;
    margin: 0 auto;
    width: 100%;
    padding: var(--r-5) var(--r-6) var(--r-7);
  }

  .head {
    display: flex;
    gap: var(--r-4);
    align-items: flex-start;
  }
  .back {
    color: var(--ink-2);
    text-decoration: none;
    font-size: 18px;
    padding: 4px 8px 0 0;
    flex: none;
    transition: color var(--t-fast);
  }
  .back:hover {
    color: var(--accent-bright);
    text-decoration: none;
  }
  .titles {
    min-width: 0;
    flex: 1;
  }
  h1 {
    font-size: 21px;
    letter-spacing: -0.015em;
    overflow-wrap: anywhere;
  }
  .ident {
    margin-top: 4px;
    font-size: 12.5px;
    color: var(--ink-2);
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: baseline;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: 12px;
  }
  .dim {
    color: var(--ink-2);
  }
  .sep {
    color: var(--ink-2);
  }
  .actions {
    display: flex;
    gap: var(--r-2);
    align-items: center;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .more {
    display: flex;
    gap: 6px;
  }
  .btn-live {
    border-color: rgba(139, 108, 246, 0.5);
    color: var(--accent-bright);
    background: var(--st-running-dim);
  }

  .stage-row {
    display: flex;
    align-items: center;
    gap: var(--r-3) var(--r-4);
    flex-wrap: wrap;
    margin: var(--r-5) 0 var(--r-4);
    padding: var(--r-3) var(--r-4);
    background: var(--bg-1);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
  }

  .human {
    display: flex;
    gap: var(--r-4);
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    border: 1px solid rgba(242, 85, 90, 0.45);
    background:
      radial-gradient(400px 120px at 0% 0%, rgba(242, 85, 90, 0.1), transparent),
      var(--bg-1);
    border-radius: var(--radius-m);
    padding: var(--r-4) var(--r-5);
    margin-bottom: var(--r-5);
  }
  .human-eyebrow {
    color: #ff8d91;
    margin-bottom: 4px;
  }
  .human-text {
    color: var(--ink-1);
    font-size: 13.5px;
    max-width: 62ch;
  }
  .human-text code {
    color: #ff8d91;
    font-size: 0.9em;
  }
  .human-actions {
    display: flex;
    gap: var(--r-2);
  }

  .tabs {
    display: flex;
    gap: 2px;
    border-bottom: 1px solid var(--line);
    overflow-x: auto;
    scrollbar-width: none;
    position: relative;
  }
  .tabs::-webkit-scrollbar {
    display: none;
  }
  /* a scrollability hint at the right edge — the strip scrolls on phones */
  .tabs::after {
    content: '';
    position: sticky;
    right: 0;
    top: 0;
    flex: none;
    width: 28px;
    align-self: stretch;
    background: linear-gradient(90deg, transparent, var(--bg-0) 70%);
    pointer-events: none;
    margin-left: -28px;
  }
  .tabs button {
    appearance: none;
    background: none;
    border: none;
    color: var(--ink-2);
    font: inherit;
    font-size: 13.5px;
    font-weight: 530;
    padding: 8px 14px 10px;
    cursor: pointer;
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
    white-space: nowrap;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    transition: color var(--t-fast);
  }
  .tabs button:hover {
    color: var(--ink-0);
  }
  .tabs button.active {
    color: var(--accent-bright);
    border-bottom-color: var(--accent);
  }
  .v-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--st-human);
  }
  .v-dot.approved {
    background: var(--st-done);
  }

  .body {
    padding-top: var(--r-5);
    min-height: 240px;
  }

  .loading,
  .missing {
    color: var(--ink-2);
    font-size: 14px;
    padding: var(--r-4) 0;
  }
  .missing .mono {
    color: var(--st-open);
    margin-bottom: 4px;
    display: block;
  }

  .file-actions {
    margin-top: var(--r-4);
    padding-top: var(--r-4);
    border-top: 1px solid var(--line);
  }

  .verdict-line {
    margin-top: var(--r-4);
    font-family: var(--font-mono);
    font-size: 12.5px;
    padding: 8px 12px;
    border-radius: var(--radius-s);
  }
  .verdict-line.approved {
    color: var(--st-done);
    background: var(--st-done-dim);
  }
  .verdict-line.rejected {
    color: #ffb3b6;
    background: var(--st-human-dim);
  }

  .editor {
    display: flex;
    flex-direction: column;
    gap: var(--r-3);
  }
  .editor .textarea {
    min-height: 380px;
  }
  .editor-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--r-3);
    flex-wrap: wrap;
  }
  .editor-actions {
    display: flex;
    gap: var(--r-2);
  }
  .hint {
    font-size: 11.5px;
    color: var(--ink-2);
  }

  .launch-hint {
    font-size: 13.5px;
    color: var(--ink-1);
    margin-bottom: var(--r-4);
    line-height: 1.6;
  }
  .launch-hint code {
    color: var(--accent-bright);
  }

  .crew {
    list-style: none;
    margin: var(--r-3) 0 0;
    padding: 0;
    max-height: 300px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .crew-row {
    display: flex;
    align-items: baseline;
    gap: var(--r-3);
    width: 100%;
    text-align: left;
    background: var(--bg-1);
    border: 1px solid var(--line);
    border-radius: var(--radius-s);
    padding: 9px 12px;
    cursor: pointer;
    font: inherit;
    transition: border-color var(--t-fast), background var(--t-fast);
  }
  .crew-row:hover:not(:disabled) {
    border-color: var(--accent);
    background: var(--bg-2);
  }
  .crew-row:disabled {
    opacity: 0.6;
    cursor: wait;
  }
  .crew-name {
    font-family: var(--font-mono);
    font-size: 12.5px;
    color: var(--accent-bright);
    flex: none;
    min-width: 96px;
  }
  .crew-desc {
    font-size: 12.5px;
    color: var(--ink-2);
  }

  @media (max-width: 760px) {
    .detail {
      padding: var(--r-4) var(--r-4) var(--r-6);
    }
    .head {
      flex-wrap: wrap;
    }
    .actions {
      width: 100%;
      justify-content: flex-start;
    }
    .stage-row {
      overflow-x: auto;
    }
  }
</style>
