<!-- Pipeline — the signature element. Every job travels define → plan → -->
<!-- implement → review → finished; this is that truth drawn as a metro line. -->
<!-- mini sits in job rows; full sits in the job header with stage labels. -->
<!-- The current station carries the run state: pulsing while mg-jdi runs, -->
<!-- red when the run stopped for a human, green once finished. -->

<script lang="ts">
  import { STAGES, stageIndex } from '$lib/stage'
  import type { JdiStatus } from '$lib/api/types'

  let {
    stage,
    jdi = null,
    variant = 'full',
  }: {
    stage: string
    jdi?: JdiStatus | null
    variant?: 'full' | 'mini'
  } = $props()

  const idx = $derived(stageIndex(stage))
  const running = $derived(jdi?.state === 'running')
  const needsHuman = $derived(jdi?.state === 'stopped:needs-human')
  const finished = $derived(stage === 'finished')
</script>

<div
  class="pipeline"
  class:mini={variant === 'mini'}
  role="img"
  aria-label={`stage {stage} of define, plan, implement, review, finished`}
>
  {#each STAGES as s, i (s)}
    {#if i > 0}
      <span class="pipe-seg" class:seg-done={i <= idx}></span>
    {/if}
    <span class="pipe-station">
      <span
        class="pipe-node"
        class:passed={i < idx}
        class:current={i === idx}
        class:pending={i > idx}
        class:is-running={i === idx && running}
        class:is-human={i === idx && needsHuman}
        class:is-done={finished || i < idx}
      ></span>
      {#if variant === 'full'}
        <span class="pipe-label" class:label-active={i === idx}>{s}</span>
      {/if}
    </span>
  {/each}
</div>

<style>
  .pipeline {
    display: flex;
    align-items: flex-start;
  }
  /* mini: real dimensions, not a transform — the layout box must shrink with
     the visuals or the row starves its text columns. */
  .pipeline.mini .pipe-node {
    width: 8px;
    height: 8px;
  }
  .pipeline.mini .pipe-node.current {
    width: 11px;
    height: 11px;
  }
  .pipeline.mini .pipe-station {
    height: 11px;
  }
  .pipeline.mini .pipe-seg {
    width: 16px;
  }

  .pipe-station {
    position: relative;
    display: flex;
    align-items: center;
    height: 13px; /* node height — keeps the line vertically centered */
  }

  .pipe-node {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    flex: none;
    border: 1.5px solid var(--ink-3);
    background: var(--bg-1);
    transition:
      border-color var(--t-fast),
      background var(--t-fast),
      box-shadow var(--t-fast);
  }

  .pipe-node.passed,
  .pipe-node.is-done {
    border-color: var(--accent);
    background: var(--accent);
    opacity: 0.85;
  }

  .pipe-node.current {
    width: 13px;
    height: 13px;
    border-color: var(--st-open);
    background: var(--bg-0);
    box-shadow: 0 0 0 3px var(--st-open-dim);
  }

  .pipe-node.current.is-done {
    border-color: var(--st-done);
    box-shadow: 0 0 0 3px var(--st-done-dim);
  }

  .pipe-node.is-running {
    border-color: var(--st-running);
    animation: pulse 1.6s ease-in-out infinite;
  }

  .pipe-node.is-human {
    border-color: var(--st-human);
    background: var(--st-human);
    box-shadow: 0 0 0 3px var(--st-human-dim);
  }

  .pipe-seg {
    width: 22px;
    height: 1.5px;
    background: var(--line-strong);
    flex: none;
    align-self: center;
    transition: background var(--t-fast);
  }
  .pipe-seg.seg-done {
    background: var(--accent);
    opacity: 0.8;
  }

  .pipe-label {
    position: absolute;
    top: 19px;
    left: 50%;
    transform: translateX(-50%);
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.08em;
    color: var(--ink-2);
    white-space: nowrap;
  }
  .pipe-label.label-active {
    color: var(--ink-0);
    font-weight: 500;
  }

  .pipeline:not(.mini) .pipe-station {
    min-width: 62px; /* reserve the label's width so it never overflows */
    justify-content: center;
  }
  .pipeline:not(.mini) {
    padding-bottom: 20px; /* room for labels without pushing the timeline too high */
  }
  .pipeline:not(.mini) .pipe-seg {
    width: 20px;
  }

  @keyframes pulse {
    0%,
    100% {
      box-shadow: 0 0 0 3px var(--st-running-dim);
    }
    50% {
      box-shadow: 0 0 0 7px rgba(139, 108, 246, 0.05);
    }
  }
</style>
