// MarkdownView — rendered, sanitized markdown for the four-file contract.

<script lang="ts">
  import { preprocessJobMarkdown, renderMarkdown } from '$lib/markdown'

  let { source }: { source: string } = $props()

  const html = $derived(renderMarkdown(preprocessJobMarkdown(source)))
</script>

<div class="md">
  <!-- Sanitized upstream in renderMarkdown (DOMPurify). -->
  {@html html}
</div>

<style>
  .md {
    font-size: 15px;
    line-height: 1.65;
    color: var(--ink-0);
    overflow-wrap: break-word;
  }

  .md :global(h1) {
    font-size: 22px;
    margin: 0 0 var(--r-4);
    letter-spacing: -0.015em;
  }
  .md :global(h2) {
    font-size: 16.5px;
    margin: var(--r-5) 0 var(--r-2);
    color: var(--ink-0);
  }
  .md :global(h3) {
    font-size: 14.5px;
    margin: var(--r-4) 0 var(--r-2);
    color: var(--ink-1);
  }
  .md :global(h2::after) {
    content: '';
    display: block;
    width: 34px;
    height: 2px;
    background: var(--accent);
    opacity: 0.55;
    border-radius: 2px;
    margin-top: 7px;
  }
  .md :global(p) {
    margin: 0 0 var(--r-3);
    color: var(--ink-1);
  }
  .md :global(strong) {
    color: var(--ink-0);
    font-weight: 600;
  }
  .md :global(p > strong:first-child) {
    /* TASK-n markers — mono, accent, the task currency of the workflow */
    font-family: var(--font-mono);
    font-size: 0.88em;
    color: var(--accent-bright);
    letter-spacing: 0.02em;
  }
  .md :global(ul),
  .md :global(ol) {
    margin: 0 0 var(--r-3);
    padding-left: 22px;
    color: var(--ink-1);
  }
  .md :global(li) {
    margin-bottom: 6px;
  }
  .md :global(li::marker) {
    color: var(--ink-3);
  }
  .md :global(code) {
    background: var(--bg-2);
    border: 1px solid var(--line);
    border-radius: 4px;
    padding: 1px 5px;
    font-size: 0.86em;
    color: #c9bdf0;
  }
  .md :global(pre) {
    background: var(--bg-1);
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
    padding: var(--r-3) var(--r-4);
    overflow-x: auto;
    margin: 0 0 var(--r-3);
  }
  .md :global(pre code) {
    background: none;
    border: none;
    padding: 0;
    font-size: 12.5px;
    line-height: 1.6;
    color: var(--ink-1);
  }
  .md :global(table) {
    border-collapse: collapse;
    margin: 0 0 var(--r-4);
    font-size: 13px;
  }
  .md :global(th),
  .md :global(td) {
    border: 1px solid var(--line);
    padding: 5px 12px;
    text-align: left;
  }
  .md :global(th) {
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-2);
    font-weight: 500;
  }
  .md :global(td) {
    color: var(--ink-1);
  }
  .md :global(td:first-child code) {
    color: var(--accent-bright);
  }
  .md :global(blockquote) {
    margin: 0 0 var(--r-3);
    border-left: 2px solid var(--accent);
    padding: 2px 0 2px var(--r-4);
    color: var(--ink-2);
  }
  .md :global(hr) {
    border: none;
    border-top: 1px solid var(--line);
    margin: var(--r-4) 0;
  }
  .md :global(a) {
    text-underline-offset: 3px;
  }
</style>
