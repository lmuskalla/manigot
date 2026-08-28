<!-- Settings — where the daemon lives and how to authenticate. The token is -->
<!-- configured out-of-band (mg serve-token / $MG_SERVE_TOKEN); the UI only -->
<!-- remembers it. Never rendered back in full after saving. -->

<script lang="ts">
  import Modal from './Modal.svelte'
  import { connection } from '$lib/state/connection.svelte'
  import { toasts } from '$lib/state/toasts.svelte'

  let { onclose }: { onclose: () => void } = $props()

  let baseUrl = $state(connection.baseUrl)
  let token = $state(connection.token)
  let busy = $state(false)

  async function save() {
    busy = true
    connection.set(baseUrl, token)
    const ok = await connection.check()
    busy = false
    if (ok) {
      toasts.ok('Connected to the daemon')
      onclose()
    } else {
      toasts.error('Could not reach the daemon — check the URL')
    }
  }
</script>

<Modal title="Connection" {busy} {onclose} width="460px">
  <div class="field">
    <label for="conn-url">daemon URL</label>
    <input
      id="conn-url"
      class="input"
      bind:value={baseUrl}
      placeholder="http://127.0.0.1:8080"
      spellcheck="false"
      autocomplete="off"
    />
    <p class="hint">Where <code>mg serve</code> listens. Leave empty when this page is served by the daemon itself.</p>
  </div>
  <div class="field">
    <label for="conn-token">bearer token</label>
    <input
      id="conn-token"
      class="input"
      bind:value={token}
      type="password"
      placeholder="tokenless (localhost)"
      spellcheck="false"
      autocomplete="off"
    />
    <p class="hint">
      A non-loopback bind requires a token (<code>mg serve-token</code> writes one to the daemon's
      .env). Over the open internet, put the daemon behind TLS.
    </p>
  </div>
  {#snippet footer()}
    <button class="btn" onclick={onclose} disabled={busy}>Cancel</button>
    <button class="btn btn-primary" onclick={save} disabled={busy}>
      {#if busy}<span class="spin" aria-hidden="true"></span>{/if}
      Save and connect
    </button>
  {/snippet}
</Modal>

<style>
  .hint {
    font-size: 12.5px;
    color: var(--ink-2);
    margin: 0;
    line-height: 1.5;
  }
  .hint code {
    color: var(--ink-1);
  }
</style>
