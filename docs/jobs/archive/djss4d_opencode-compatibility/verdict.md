# Verdict: opencode-compatibility

id: djss4d
status: passed
reviewer:
date: 2026-08-08

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

<!-- TASK-1: PASS / FAIL / PARTIAL
     notes: ...

TASK-2: ...
-->

## Security

<!-- Any security findings from @security, or "none" if not run. -->

## Overall

APPROVED.

Verified end-to-end with a real Z.AI Coding Plan: `ZHIPU_API_KEY` is forwarded
into the container (added to the key allowlist in `run.sh` + `entrypoint.sh`),
and OpenCode now boots into `zai-coding-plan/glm-5.2` on the Coding Plan
endpoint. The `OPENCODE_MODEL` var is consumed via a generated config
(`{env:OPENCODE_MODEL}`), since OpenCode ignores the bare env var — without it
the container defaulted to a Zhipu pay-per-token model.

One follow-up remains (non-blocking): the provider-key allowlist is duplicated
in `run.sh` and `entrypoint.sh`; both carry a keep-in-sync comment.
