# Setting up ntfy connectivity

manigot can push ntfy notifications for `mg jdi` runs. This document walks
through what to configure on the host (the machine running `mg`) and what to
configure on the ntfy server side.

## How the feature works

`mg jdi` pushes to ntfy on three events:

1. A run **finishes** — success notification (tag `white_check_mark`)
2. A run **stops needing a human** (verdict rejected or `NEEDS-HUMAN-INPUT:`)
   — high-priority attention notification (tag `warning`, priority 4)
3. A later run **starts** and finds the previous run crashed/killed (stale
   `running` sidecar)

Each publish is a plain `POST {NTFY_URL}/{NTFY_TOPIC}` with the message as the
body and `Title`, `Priority`, `Tags` as HTTP headers, plus
`Authorization: Bearer <token>` if you configure one. 10s timeout, and a
failed send is only a stderr warning — it never aborts the run.

## Host side (the machine running `mg`)

There are exactly three keys, all in the manigot checkout's `.env`
(gitignored):

| Key | Meaning | Default |
|---|---|---|
| `NTFY_URL` | Your ntfy server base URL, no trailing slash | `https://ntfy.sh` |
| `NTFY_TOPIC` | Topic to publish to — **the activation key** | *(unset = feature off)* |
| `NTFY_TOKEN` | Optional bearer token (`tk_...`) for authenticated servers | *(none)* |

`NTFY_TOPIC` empty/unset = strict no-op, byte-for-byte identical behavior to
no feature. `NTFY_TOKEN` is optional — only needed if your server has auth
enabled.

### Option A — via the wizard (recommended)

```
mg setup
```

Run **with no profile name**. The bare wizard walks through the three
profiles *and then* the ntfy block, prompting `NTFY_URL` (prefilled
`https://ntfy.sh`), `NTFY_TOPIC`, and `NTFY_TOKEN` (masked secret prompt).
"Leave NTFY_TOPIC empty to keep notifications off."

Two caveats:

- `mg setup <profile>` (with a profile) skips the ntfy block — it only runs
  in the no-profile walk.
- `mg setup --check` does **not** cover ntfy (its check shape is
  profile-shaped). So `--check` will never tell you about ntfy — that's
  expected.

### Option B — hand-edit `manigot/.env`

The file lives in the manigot checkout that `mg` actually resolves
(`$MANIGOT_HOME`, else the directory the `mg` binary lives in, else `$PWD`).
Add:

```
NTFY_URL=https://ntfy.yourdomain.com
NTFY_TOPIC=manigot-jdi-alerts
NTFY_TOKEN=tk_xxxxxxxxxxxx
```

Process env overrides work too: if the key isn't in `.env`, `EnvValue` falls
back to the inherited environment — handy for a quick test with
`NTFY_TOPIC=... mg jdi -j ...`.

## ntfy server side (your existing server)

Four things to check/do, in order:

### 1. Reachability — what `NTFY_URL` must point at

The client posts to `{NTFY_URL}/{NTFY_TOPIC}`. So:

- If the server is HTTPS and internet-reachable: `https://ntfy.example.com` —
  verify `curl -s https://ntfy.example.com/v1/health` returns `200 OK`
  **from the host running `mg jdi`** (it's often a different box than your
  server).
- If it's a LAN box with plain HTTP: `http://192.168.1.50:80` works fine —
  nothing in the client requires TLS.
- Check the host's firewall/egress to the ntfy port (80/443), and that your
  reverse proxy (Caddy/nginx) forwards `POST /{topic}` — no special paths,
  just the topic root.

### 2. Topic

On a self-hosted server with auth **off**, topics are wide open — anyone who
can reach the server can read and write any topic. Treat the topic name as
the password: use something unguessable, e.g. `manigot-7f3k9q2x-jdi`.

### 3. Authentication (only if you enable it — recommended for internet-facing servers)

This is the one place to be careful, because the manigot client only speaks
the **bearer access-token** form, not basic auth (`user:password`). On the
server:

```
sudo ntfy user add --role=admin manigot          # or a non-admin user
sudo ntfy access allow manigot 'manigot-*' rw    # scope to your topic/pattern
sudo ntfy token add --label=mg-jdi manigot
```

The last command prints a `tk_...` string — **that** is your `NTFY_TOKEN`. If
you instead hand out a username/password, manigot can't use it (it sends
`Authorization: Bearer ...`, never basic auth). If you're on ntfy ≥2 and
already have auth enabled, `ntfy token add` is available out of the box.

### 4. Where the notification actually lands (the part people forget)

ntfy server-side config only makes the *publish* succeed. To *receive* it you
need a subscriber:

- **Phone**: ntfy Android/iOS app → add your server
  (`https://ntfy.example.com`) → subscribe to the topic (authenticate with
  the same credentials if enabled).
- **Browser**: `https://ntfy.example.com/app` on the same topic.
- **Desktop**: the app's web page or any ntfy client.

## Verification (do this before trusting a jdi run)

**Server + topic + token**, from the host that runs `mg jdi` — this is
byte-for-byte the same request shape manigot sends (no auth variant included
if you have no token):

```
curl -s -d "test from mg jdi" \
     -H "Title: mg jdi test" \
     -H "Priority: 3" \
     -H "Tags: test" \
     [-H "Authorization: Bearer tk_..." ] \
     https://ntfy.example.com/manigot-7f3k9q2x-jdi
```

A `{"id": ...}` JSON response means the publish landed; the message should pop
on your subscribed device. A 401/403 means the token/ACL is wrong.

**Then** run a real `mg jdi --job <id>` — you'll see the success/attention
notifications, and if the publish ever fails you'll get
`mg jdi: warning: could not send ntfy notification: ...` on stderr
(deliberately redacted — never includes the URL/topic/token).

## What you don't need to do

- Nothing in the Docker image, `agents/`, or the job workflow — ntfy is
  host-side only.
- No firewall rule changes on the *client* host beyond outbound HTTPS.
