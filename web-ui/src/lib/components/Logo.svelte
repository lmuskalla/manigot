<!-- The manigot mark — three agent cells on a rail, redrawn as SVG from the -->
<!-- ASCII logo (assets/manigot.txt). The cells light up in the status hues: -->
<!-- amber (work to define), green (done), accent (in flight). -->

<script lang="ts">
  let { size = 22 }: { size?: number } = $props()
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 32 32"
  fill="none"
  aria-hidden="true"
>
  <rect x="5.2" y="10.2" width="7.2" height="7.2" rx="1.8" stroke="var(--accent)" stroke-width="1.7" />
  <rect x="12.4" y="10.2" width="7.2" height="7.2" rx="1.8" stroke="var(--accent)" stroke-width="1.7" opacity="0.75" />
  <rect x="19.6" y="10.2" width="7.2" height="7.2" rx="1.8" stroke="var(--accent)" stroke-width="1.7" opacity="0.5" />
  <circle cx="8.8" cy="13.8" r="1.5" fill="var(--st-open)" />
  <circle cx="16" cy="13.8" r="1.5" fill="var(--st-done)" />
  <circle cx="23.2" cy="13.8" r="1.5" fill="var(--accent-bright)" />
</svg>
