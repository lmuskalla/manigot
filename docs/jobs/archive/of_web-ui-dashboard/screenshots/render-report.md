# Render report: http://localhost:5173/?mock=1#/

- captured: 2026-08-29T12:57:23.803Z
- widths: 375, 1280

## Findings — errors

- **[overflow]** span.name.s-1uFe6eOMGzLc"harbor_needs-human-on-conflict-handling": internal-horizontal-overflow: scrollWidth 273 > clientWidth 227 (overflow-x: hidden)
- **[overflow]** span.title.s-1uFe6eOMGzLc"listener mutating API + run supervision": internal-horizontal-overflow: scrollWidth 250 > clientWidth 227 (overflow-x: hidden)
- **[overflow]** span.title.s-1uFe6eOMGzLc"rate limit headers missing on 429": internal-horizontal-overflow: scrollWidth 214 > clientWidth 205 (overflow-x: hidden)
- **[overflow]** span.title.s-1uFe6eOMGzLc"prune orphaned containers on a timer": internal-horizontal-overflow: scrollWidth 243 > clientWidth 227 (overflow-x: hidden)
- **[overflow]** span.title.s-1uFe6eOMGzLc"TUI bell rings at full volume": internal-horizontal-overflow: scrollWidth 175 > clientWidth 173 (overflow-x: hidden)
- **[overflow]** span.name.s-1uFe6eOMGzLc"lumen_doctor-health-check": internal-horizontal-overflow: scrollWidth 175 > clientWidth 173 (overflow-x: hidden)

## Findings — warnings

- **[contrast-aaa]** p "overview": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "projects": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "open jobs": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "needs human": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "running": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "harbor_needs-human-on-conflict-handling": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "14 min ago": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "farmer_part-2-of-web-ui-tui-path": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "developer running": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "2 min ago": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "solyto-api": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "cedar_rate-limit-headers": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "analyst running": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "just now": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "quartz_prune-on-a-timer": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "2026-08-26": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "ember_tui-bell-volume": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "3 h ago": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "lumen_doctor-health-check": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "1 d ago": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "5 jobs": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "1 job": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** a "Jobs": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** a "Crew": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** a "Daemon": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[overlap]** div.page.s-1uFe6eOMGzLc"overview Dashboard 2 projects 6 open job" × nav.tabbar.s-XsEmFtvddWTw"Jobs Crew Daemon": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** div.page.s-1uFe6eOMGzLc"overview Dashboard 2 projects 6 open job" × a.s-XsEmFtvddWTw"Jobs": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** div.page.s-1uFe6eOMGzLc"overview Dashboard 2 projects 6 open job" × a.s-XsEmFtvddWTw"Crew": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** div.page.s-1uFe6eOMGzLc"overview Dashboard 2 projects 6 open job" × a.s-XsEmFtvddWTw"Daemon": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** section.block.s-1uFe6eOMGzLc"Needs attention manigot conflict handlin" × nav.tabbar.s-XsEmFtvddWTw"Jobs Crew Daemon": rects overlap 91% of the smaller (z-index 0 vs 0)
- **[overlap]** section.block.s-1uFe6eOMGzLc"Needs attention manigot conflict handlin" × a.s-XsEmFtvddWTw"Jobs": rects overlap 87% of the smaller (z-index 0 vs 0)
- **[overlap]** section.block.s-1uFe6eOMGzLc"Needs attention manigot conflict handlin" × a.s-XsEmFtvddWTw"Crew": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** section.block.s-1uFe6eOMGzLc"Needs attention manigot conflict handlin" × a.s-XsEmFtvddWTw"Daemon": rects overlap 87% of the smaller (z-index 0 vs 0)
- **[overlap]** ul.jobs.s-1uFe6eOMGzLc"manigot conflict handling over HTTP harb" × nav.tabbar.s-XsEmFtvddWTw"Jobs Crew Daemon": rects overlap 91% of the smaller (z-index 0 vs 0)
- **[overlap]** ul.jobs.s-1uFe6eOMGzLc"manigot conflict handling over HTTP harb" × a.s-XsEmFtvddWTw"Jobs": rects overlap 87% of the smaller (z-index 0 vs 0)
- **[overlap]** ul.jobs.s-1uFe6eOMGzLc"manigot conflict handling over HTTP harb" × a.s-XsEmFtvddWTw"Crew": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** ul.jobs.s-1uFe6eOMGzLc"manigot conflict handling over HTTP harb" × a.s-XsEmFtvddWTw"Daemon": rects overlap 87% of the smaller (z-index 0 vs 0)
- **[overlap]** li.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × nav.tabbar.s-XsEmFtvddWTw"Jobs Crew Daemon": rects overlap 91% of the smaller (z-index 0 vs 0)
- **[overlap]** li.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × a.s-XsEmFtvddWTw"Jobs": rects overlap 87% of the smaller (z-index 0 vs 0)
- **[overlap]** li.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × a.s-XsEmFtvddWTw"Crew": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** li.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × a.s-XsEmFtvddWTw"Daemon": rects overlap 87% of the smaller (z-index 0 vs 0)
- **[overlap]** a.job.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × nav.tabbar.s-XsEmFtvddWTw"Jobs Crew Daemon": rects overlap 91% of the smaller (z-index 0 vs 0)
- **[overlap]** a.job.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × a.s-XsEmFtvddWTw"Jobs": rects overlap 87% of the smaller (z-index 0 vs 0)
- **[overlap]** a.job.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × a.s-XsEmFtvddWTw"Crew": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** a.job.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × a.s-XsEmFtvddWTw"Daemon": rects overlap 87% of the smaller (z-index 0 vs 0)
- **[overlap]** div.left.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × nav.tabbar.s-XsEmFtvddWTw"Jobs Crew Daemon": rects overlap 81% of the smaller (z-index 0 vs 0)
- **[overlap]** div.left.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × a.s-XsEmFtvddWTw"Jobs": rects overlap 53% of the smaller (z-index 0 vs 0)
- **[overlap]** div.left.s-1uFe6eOMGzLc"manigot mg doctor health check lumen_doc" × a.s-XsEmFtvddWTw"Crew": rects overlap 73% of the smaller (z-index 0 vs 0)
- **[overlap]** span.proj.s-1uFe6eOMGzLc"manigot" × nav.tabbar.s-XsEmFtvddWTw"Jobs Crew Daemon": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** span.proj.s-1uFe6eOMGzLc"manigot" × a.s-XsEmFtvddWTw"Jobs": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** div.ident.s-1uFe6eOMGzLc"mg doctor health check lumen_doctor-heal" × nav.tabbar.s-XsEmFtvddWTw"Jobs Crew Daemon": rects overlap 81% of the smaller (z-index 0 vs 0)
- **[overlap]** div.ident.s-1uFe6eOMGzLc"mg doctor health check lumen_doctor-heal" × a.s-XsEmFtvddWTw"Crew": rects overlap 73% of the smaller (z-index 0 vs 0)
- **[overlap]** span.title.s-1uFe6eOMGzLc"mg doctor health check" × nav.tabbar.s-XsEmFtvddWTw"Jobs Crew Daemon": rects overlap 100% of the smaller (z-index 0 vs 0)
- **[overlap]** span.title.s-1uFe6eOMGzLc"mg doctor health check" × a.s-XsEmFtvddWTw"Crew": rects overlap 72% of the smaller (z-index 0 vs 0)
- **[contrast-aaa]** span "control plane": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** label "project": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** button "commands": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** p "overview": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "projects": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "open jobs": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "needs human": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "running": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "harbor_needs-human-on-conflict-handling": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "14 min ago": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "farmer_part-2-of-web-ui-tui-path": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "developer running": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "2 min ago": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "solyto-api": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "cedar_rate-limit-headers": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "analyst running": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "just now": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "quartz_prune-on-a-timer": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "2026-08-26": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "ember_tui-bell-volume": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "3 h ago": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "manigot": contrast 6.87:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "lumen_doctor-health-check": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "1 d ago": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "5 jobs": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)
- **[contrast-aaa]** span "1 job": contrast 5.45:1 — passes AA but fails AAA (needs 7:1)

## Viewport 375px

- viewport: 375×900, DPR 1
- document: 375×900 (client 375px)
- fonts: loaded
- spacing scale: 2, 6, 12, 16, 32

### Element inventory

| element | text | rect | font | contrast |
|---|---|---|---|---|
| p.eyebrow | "overview" | 16,16 343×16 | 10.5px 400 | 5.45:1 AA |
| h1.s-1uFe6eOMGzLc | "Dashboard" | 16,34 343×30 | 24px 640 | 15.49:1 AA/AAA |
| span.stat-v | "2" | 41,97 116×40 | 26px 660 | 15.49:1 AA/AAA |
| span.stat-k | "projects" | 41,140 116×17 | 11px 400 | 5.45:1 AA |
| span.stat-v | "6" | 219,97 116×40 | 26px 660 | 15.49:1 AA/AAA |
| span.stat-k | "open jobs" | 219,140 116×17 | 11px 400 | 5.45:1 AA |
| span.stat-v | "1" | 41,203 116×40 | 26px 660 | 5.54:1 AA/AAA |
| span.stat-k | "needs human" | 41,245 116×17 | 11px 400 | 5.45:1 AA |
| span.stat-v | "2" | 219,203 116×40 | 26px 660 | 15.49:1 AA/AAA |
| span.stat-k | "running" | 219,245 116×17 | 11px 400 | 5.45:1 AA |
| h2.s-1uFe6eOMGzLc | "Needs attention" | 16,311 343×19 | 15.5px 640 | 15.49:1 AA/AAA |
| a.job | (interactive) | 16,342 343×103 | 15px 400 | — |
| span.proj | "manigot" | 33,364 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "conflict handling over HTTP" | 115,355 227×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "harbor_needs-human-on-conflict" | 115,378 227×18 | 11.5px 400 | 5.45:1 AA |
| span.chip | "needs human" | 33,408 112×25 | 11px 400 | 8.43:1 AA/AAA |
| span.when | "14 min ago" | 153,411 58×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 16,453 343×103 | 15px 400 | — |
| span.proj | "manigot" | 33,475 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "listener mutating API + run su" | 115,466 227×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "farmer_part-2-of-web-ui-tui-pa" | 115,489 227×18 | 11.5px 400 | 5.45:1 AA |
| span.chip | "developer running" | 33,519 156×25 | 11px 400 | 6.87:1 AA |
| span.when | "2 min ago" | 197,522 51×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 16,564 343×103 | 15px 400 | — |
| span.proj | "solyto-api" | 33,586 88×23 | 11px 400 | 6.87:1 AA |
| span.title | "rate limit headers missing on " | 137,577 205×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "cedar_rate-limit-headers" | 137,600 205×18 | 11.5px 400 | 5.45:1 AA |
| span.chip | "analyst running" | 33,630 142×25 | 11px 400 | 6.87:1 AA |
| span.when | "just now" | 183,633 43×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 16,675 343×97 | 15px 400 | — |
| span.proj | "manigot" | 33,697 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "prune orphaned containers on a" | 115,688 227×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "quartz_prune-on-a-timer" | 115,711 227×18 | 11.5px 400 | 5.45:1 AA |
| span.when | "2026-08-26" | 33,741 64×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 16,780 343×66 | 15px 400 | — |
| span.proj | "manigot" | 33,802 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "TUI bell rings at full volume" | 115,793 173×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "ember_tui-bell-volume" | 115,816 173×18 | 11.5px 400 | 5.45:1 AA |
| span.when | "3 h ago" | 304,804 38×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 16,854 343×66 | 15px 400 | — |
| span.proj | "manigot" | 33,876 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "mg doctor health check" | 115,867 173×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "lumen_doctor-health-check" | 115,890 173×18 | 11.5px 400 | 5.45:1 AA |
| span.when | "1 d ago" | 304,878 38×19 | 12px 400 | 5.45:1 AA |
| h2.s-1uFe6eOMGzLc | "Projects" | 16,953 343×19 | 15.5px 640 | 15.49:1 AA/AAA |
| a.proj-card | (interactive) | 16,984 343×83 | 15px 400 | — |
| span.proj-name | "manigot" | 33,1001 309×23 | 15px 600 | 15.49:1 AA/AAA |
| span.proj-count | "5 jobs" | 33,1030 309×19 | 12.5px 400 | 5.45:1 AA |
| a.proj-card | (interactive) | 16,1079 343×83 | 15px 400 | — |
| span.proj-name | "solyto-api" | 33,1096 309×23 | 15px 600 | 15.49:1 AA/AAA |
| span.proj-count | "1 job" | 33,1125 309×19 | 12.5px 400 | 5.45:1 AA |
| a.s-XsEmFtvddWTw | "Jobs" | 0,855 125×45 | 13.5px 560 | 5.45:1 AA |
| a.s-XsEmFtvddWTw | "Crew" | 125,855 125×45 | 13.5px 560 | 5.45:1 AA |
| a.s-XsEmFtvddWTw | "Daemon" | 250,855 125×45 | 13.5px 560 | 5.45:1 AA |

## Viewport 1280px

- viewport: 1280×900, DPR 1
- document: 1280×900 (client 1280px)
- fonts: loaded
- spacing scale: 2, 6, 8, 12, 16, 24, 32, 536

### Element inventory

| element | text | rect | font | contrast |
|---|---|---|---|---|
| a.brand | (interactive) | 16,24 199×26 | 15px 400 | — |
| span.word | "manigot" | 49,24 57×26 | 16.5px 680 | 15.49:1 AA/AAA |
| span.role | "control plane" | 113,28 95×18 | 9.5px 400 | 5.45:1 AA |
| label.eyebrow | "project" | 16,74 199×16 | 10.5px 400 | 5.45:1 AA |
| select#project-select | (interactive) | 16,96 199×40 | 14px 400 | — |
| a.s-XsEmFtvddWTw | "Jobs" | 16,160 199×36 | 14px 520 | 8.53:1 AA/AAA |
| a.s-XsEmFtvddWTw | "Crew" | 16,197 199×36 | 14px 520 | 8.53:1 AA/AAA |
| a.s-XsEmFtvddWTw | "Daemon" | 16,235 199×36 | 14px 520 | 8.53:1 AA/AAA |
| button.conn | "connected" | 16,806 199×33 | 12.5px 400 | 8.53:1 AA/AAA |
| button.palette-hint | "commands" | 16,847 199×29 | 12px 400 | 5.45:1 AA |
| span.kbd | "⌘K" | 18,851 25×21 | 10.5px 400 | 8.53:1 AA/AAA |
| p.eyebrow | "overview" | 264,32 984×16 | 10.5px 400 | 5.45:1 AA |
| h1.s-1uFe6eOMGzLc | "Dashboard" | 264,50 984×30 | 24px 640 | 15.49:1 AA/AAA |
| span.stat-v | "2" | 289,113 187×40 | 26px 660 | 15.49:1 AA/AAA |
| span.stat-k | "projects" | 289,156 187×17 | 11px 400 | 5.45:1 AA |
| span.stat-v | "6" | 538,113 187×40 | 26px 660 | 15.49:1 AA/AAA |
| span.stat-k | "open jobs" | 538,156 187×17 | 11px 400 | 5.45:1 AA |
| span.stat-v | "1" | 787,113 187×40 | 26px 660 | 5.54:1 AA/AAA |
| span.stat-k | "needs human" | 787,156 187×17 | 11px 400 | 5.45:1 AA |
| span.stat-v | "2" | 1036,113 187×40 | 26px 660 | 15.49:1 AA/AAA |
| span.stat-k | "running" | 1036,156 187×17 | 11px 400 | 5.45:1 AA |
| h2.s-1uFe6eOMGzLc | "Needs attention" | 264,222 984×19 | 15.5px 640 | 15.49:1 AA/AAA |
| a.job | (interactive) | 264,253 984×66 | 15px 400 | — |
| span.proj | "manigot" | 281,275 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "conflict handling over HTTP" | 363,266 273×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "harbor_needs-human-on-conflict" | 363,288 273×18 | 11.5px 400 | 5.45:1 AA |
| span.chip | "needs human" | 1053,274 112×25 | 11px 400 | 8.43:1 AA/AAA |
| span.when | "14 min ago" | 1173,277 58×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 264,327 984×66 | 15px 400 | — |
| span.proj | "manigot" | 281,349 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "listener mutating API + run su" | 363,340 250×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "farmer_part-2-of-web-ui-tui-pa" | 363,363 250×18 | 11.5px 400 | 5.45:1 AA |
| span.chip | "developer running" | 1016,348 156×25 | 11px 400 | 6.87:1 AA |
| span.when | "2 min ago" | 1180,351 51×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 264,402 984×66 | 15px 400 | — |
| span.proj | "solyto-api" | 281,423 88×23 | 11px 400 | 6.87:1 AA |
| span.title | "rate limit headers missing on " | 385,415 214×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "cedar_rate-limit-headers" | 385,437 214×18 | 11.5px 400 | 5.45:1 AA |
| span.chip | "analyst running" | 1038,422 142×25 | 11px 400 | 6.87:1 AA |
| span.when | "just now" | 1188,425 43×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 264,476 984×66 | 15px 400 | — |
| span.proj | "manigot" | 281,497 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "prune orphaned containers on a" | 363,489 243×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "quartz_prune-on-a-timer" | 363,511 243×18 | 11.5px 400 | 5.45:1 AA |
| span.when | "2026-08-26" | 1167,500 64×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 264,550 984×66 | 15px 400 | — |
| span.proj | "manigot" | 281,572 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "TUI bell rings at full volume" | 363,563 175×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "ember_tui-bell-volume" | 363,586 175×18 | 11.5px 400 | 5.45:1 AA |
| span.when | "3 h ago" | 1193,574 38×19 | 12px 400 | 5.45:1 AA |
| a.job | (interactive) | 264,624 984×66 | 15px 400 | — |
| span.proj | "manigot" | 281,646 66×23 | 11px 400 | 6.87:1 AA |
| span.title | "mg doctor health check" | 363,637 175×22 | 14.5px 590 | 15.49:1 AA/AAA |
| span.name | "lumen_doctor-health-check" | 363,660 175×18 | 11.5px 400 | 5.45:1 AA |
| span.when | "1 d ago" | 1193,648 38×19 | 12px 400 | 5.45:1 AA |
| h2.s-1uFe6eOMGzLc | "Projects" | 264,723 984×19 | 15.5px 640 | 15.49:1 AA/AAA |
| a.proj-card | (interactive) | 264,754 237×83 | 15px 400 | — |
| span.proj-name | "manigot" | 281,771 203×23 | 15px 600 | 15.49:1 AA/AAA |
| span.proj-count | "5 jobs" | 281,800 203×19 | 12.5px 400 | 5.45:1 AA |
| a.proj-card | (interactive) | 513,754 237×83 | 15px 400 | — |
| span.proj-name | "solyto-api" | 530,771 203×23 | 15px 600 | 15.49:1 AA/AAA |
| span.proj-count | "1 job" | 530,800 203×19 | 12.5px 400 | 5.45:1 AA |
