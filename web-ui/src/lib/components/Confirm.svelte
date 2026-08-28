<!-- Confirm — the CLI's confirmation discipline, kept verbatim. mg delete asks -->
<!-- "This cannot be done."… no: "This cannot be undone."; mg done warns on an -->
<!-- unapproved verdict; the wording is a product asset, not an accident. -->

<script lang="ts">
  import Modal from './Modal.svelte'

  let {
    title,
    body,
    confirmLabel = 'Confirm',
    confirmKind = 'default' as 'default' | 'danger' | 'primary',
    busy = false,
    onconfirm,
    onclose,
  }: {
    title: string
    body: string
    confirmLabel?: string
    confirmKind?: 'default' | 'danger' | 'primary'
    busy?: boolean
    onconfirm: () => void
    onclose: () => void
  } = $props()
</script>

<Modal {title} {busy} {onclose} width="440px">
  <p class="body">{body}</p>
  {#snippet footer()}
    <button class="btn" onclick={onclose} disabled={busy}>Cancel</button>
    <button
      class="btn"
      class:btn-danger={confirmKind === 'danger'}
      class:btn-primary={confirmKind === 'primary'}
      onclick={onconfirm}
      disabled={busy}
    >
      {#if busy}<span class="spin" aria-hidden="true"></span>{/if}
      {confirmLabel}
    </button>
  {/snippet}
</Modal>

<style>
  .body {
    white-space: pre-line;
    line-height: 1.6;
  }
</style>
