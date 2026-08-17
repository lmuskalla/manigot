#!/usr/bin/env node
// shot — agent sight: render a URL to a PNG and produce a model-free render
// report of exact DOM facts. See docs/PLAYWRIGHT.md.
//
// Interface:
//   shot <url>                    capture at 1280x900 + render report
//   shot <url> --widths 375,768   responsive review (default 1280)
//   shot <url> --full-page        full-height capture
//   shot <url> --describe         add vision-model prose to the report
//   shot --help
//
// Output lands in <job dir>/screenshots/: one PNG per viewport width, plus
// render-report.md and render-report.json. The job dir is resolved from the
// git branch (docs/jobs/<id>_<slug>/), falling back to the current directory.
'use strict';

const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const DEFAULT_WIDTH = 1280;
const DEFAULT_HEIGHT = 900;
const ALLOWED_WITH = ['--widths', '--full-page', '--describe', '--help'];

// ── CLI parsing ──────────────────────────────────────────────────────────────

function parseArgs(argv) {
  const opts = {
    url: null,
    widths: [DEFAULT_WIDTH],
    fullPage: false,
    describe: false,
    help: false,
  };
  const positional = [];
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === '--help' || arg === '-h') {
      opts.help = true;
    } else if (arg === '--widths') {
      const v = argv[++i];
      if (!v) fail('--widths needs a value: --widths 375,768,1280');
      opts.widths = v.split(',').map(s => parseInt(s.trim(), 10)).filter(n => Number.isFinite(n) && n > 0);
      if (opts.widths.length === 0) fail('--widths needs at least one positive number: --widths 375,768,1280');
    } else if (arg === '--full-page') {
      opts.fullPage = true;
    } else if (arg === '--describe') {
      opts.describe = true;
    } else if (arg.startsWith('-')) {
      fail(`unknown option: ${arg}\nrun 'shot --help' for usage`);
    } else {
      positional.push(arg);
    }
  }
  if (!opts.help) {
    if (positional.length === 0) fail('missing <url>\nrun \'shot --help\' for usage');
    opts.url = positional[positional.length - 1];
  }
  return opts;
}

function fail(msg) {
  console.error(`shot: ${msg}`);
  process.exit(1);
}

// The Claude Code gating layer: `shot` is agent-neutral tooling, but only
// committing agents (developer) may run it — read-only agents consume the
// artifacts. The session launcher sets MANIGOT_AGENT_COMMITS=false for
// non-committing agents (the same marker that makes their gitdir mount
// read-only); OpenCode enforces the same rule via permission: blocks, so this
// guard is the soft-but-present second layer, mirroring the git shim. Unset
// defaults to true, matching agentCommits' default (a missing marker never
// breaks a committing agent).
function guardAgent() {
  if (process.env.MANIGOT_AGENT_COMMITS === 'false') {
    fail('refusing to run: the current agent is read-only (MANIGOT_AGENT_COMMITS=false). Read-only agents consume the render report and PNG; they do not run shot.');
  }
}

function usage() {
  return `shot — render a URL and measure it (agent sight). See docs/PLAYWRIGHT.md.

Usage:
  shot <url>                    capture at 1280x900; PNG + render report to screenshots/
  shot <url> --widths 375,768   capture at each viewport width (default 1280)
  shot <url> --full-page        capture the full page height
  shot <url> --describe         add vision-model prose (needs ZHIPU_API_KEY)
  shot --help                   this help

Output:
  <job dir>/screenshots/ — one PNG per width, plus render-report.md and
  render-report.json. The job dir is docs/jobs/<id>_<slug>/ (from the git
  branch), or the current directory when no job branch matches.

Report content (model-free measured layer):
  - element inventory: visible interactive/text elements, rects, computed styles
  - WCAG contrast ratios for text, AA/AAA pass/fail
  - overflow and clipping (scrollWidth vs clientWidth, beyond-viewport elements)
  - alignment / shared-grid-line and spacing-scale extraction
  - font loading status and fallback detection
  - viewport, devicePixelRatio, z-index / overlap anomalies
`;
}

// ── Job dir resolution ───────────────────────────────────────────────────────

function resolveOutDir() {
  if (process.env.SHOT_OUTDIR) return process.env.SHOT_OUTDIR;
  // In a job worktree the branch is [<prefix>/]<type>/<id>_<slug>; the job
  // dir is docs/jobs/<id>_<slug>/ under the git root. Fall back to cwd when
  // the branch doesn't name an existing job dir.
  try {
    const branch = execSync('git branch --show-current', { encoding: 'utf8' }).trim();
    const slug = branch.split('/').pop();
    if (slug) {
      const gitRoot = execSync('git rev-parse --show-toplevel', { encoding: 'utf8' }).trim();
      const jobDir = path.join(gitRoot, 'docs', 'jobs', slug);
      if (fs.existsSync(path.join(jobDir))) return jobDir;
    }
  } catch (_) { /* not a git repo or no branch — fall through */ }
  return process.cwd();
}

function urlSlug(url) {
  return url
    .replace(/^https?:\/\//, '')
    .replace(/[^a-zA-Z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 60) || 'page';
}

// ── The measured layer (runs in-page) ────────────────────────────────────────

// Returns everything the report needs for one viewport. All computation is
// done against the live DOM so the results are exact facts, not estimates.
// Runs entirely in the page context — Playwright serializes the function,
// so every helper it uses must be defined inside it.
function measurePage({ width: w, fullPage }) {

  // Luminance/contrast helpers (WCAG 2.x relative luminance).
  const lumOf = (rgb) => {
    if (!rgb) return null;
    const m = rgb.match(/\d+(\.\d+)?/g);
    if (!m || m.length < 3) return null;
    const chan = (c) => {
      const s = c / 255;
      return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
    };
    return 0.2126 * chan(Number(m[0])) + 0.7152 * chan(Number(m[1])) + 0.0722 * chan(Number(m[2]));
  };
  const contrast = (a, b) => {
    const hi = Math.max(a, b), lo = Math.min(a, b);
    return (hi + 0.05) / (lo + 0.05);
  };

  // Effective background: walk up ancestors compositing background-color until
  // opaque; default white. Returns a css color string.
  const effectiveBackground = (el) => {
    let node = el;
    const stack = [];
    while (node && node.nodeType === 1) {
      stack.push(node);
      node = node.parentElement;
    }
    stack.push(document.documentElement);
    let bg = 'rgb(255, 255, 255)';
    let alpha = 1;
    for (let i = stack.length - 1; i >= 0; i--) {
      const cs = getComputedStyle(stack[i]);
      const m = cs.backgroundColor.match(/rgba?\(([^)]+)\)/);
      if (!m) continue;
      const parts = m[1].split(',').map(s => parseFloat(s.trim()));
      const [r, g, b] = parts;
      const a = parts.length > 3 ? parts[3] : 1;
      if (a <= 0) continue;
      // Composite over the accumulated background.
      const out = (c) => Math.round(c * a + parseFloat(bg.match(/\d+/)[0]) * (1 - a));
      bg = `rgb(${out(r)}, ${out(g)}, ${out(b)})`;
      alpha = alpha * a;
      if (alpha >= 0.999) break;
    }
    return bg;
  };

  const isVisible = (el) => {
    const style = getComputedStyle(el);
    if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') return false;
    const r = el.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };

  const describeEl = (el) => {
    const parts = [el.tagName.toLowerCase()];
    if (el.id) parts.push(`#${el.id}`);
    if (el.className && typeof el.className === 'string') {
      const cls = el.className.split(/\s+/).slice(0, 2);
      if (cls.length) parts.push('.' + cls.join('.'));
    }
    const text = (el.textContent || '').trim().slice(0, 40);
    if (text) parts.push(`"${text}"`);
    return parts.join('');
  };

  // Own text = text directly in this element (not descendants).
  const ownText = (el) => {
    let txt = '';
    for (const n of el.childNodes) {
      if (n.nodeType === 3) txt += n.textContent;
    }
    return txt.trim();
  };

  const isInteractive = (el) => {
    const tag = el.tagName.toLowerCase();
    if (['a', 'button', 'input', 'select', 'textarea', 'label', 'summary', 'details'].includes(tag)) return true;
    const role = el.getAttribute('role');
    if (role && ['button', 'link', 'menuitem', 'tab', 'checkbox', 'radio', 'switch', 'combobox', 'slider'].includes(role)) return true;
    return false;
  };

  const elements = [...document.querySelectorAll('body *')].filter(isVisible);

  // ── Element inventory: text + interactive elements with computed styles ──
  const inventory = [];
  for (const el of elements) {
    const text = ownText(el);
    const interactive = isInteractive(el);
    if (!text && !interactive) continue;
    const r = el.getBoundingClientRect();
    const cs = getComputedStyle(el);
    const rect = {
      x: Math.round(r.x * 100) / 100, y: Math.round(r.y * 100) / 100,
      width: Math.round(r.width * 100) / 100, height: Math.round(r.height * 100) / 100,
    };
    const color = cs.color;
    const bg = cs.backgroundColor;
    const bgEffective = effectiveBackground(el);
    let item = {
      tag: el.tagName.toLowerCase(),
      id: el.id || undefined,
      classes: el.className && typeof el.className === 'string' ? el.className.split(/\s+/).slice(0, 4) : undefined,
      text: text ? text.slice(0, 80) : undefined,
      interactive,
      rect,
      fontSize: parseFloat(cs.fontSize),
      fontWeight: cs.fontWeight,
      lineHeight: cs.lineHeight,
      color,
      background: bg,
      backgroundEffective: bgEffective,
      fontFamily: cs.fontFamily.split(',')[0].trim(),
      zIndex: cs.zIndex,
      position: cs.position,
    };
    // Contrast for text-bearing elements.
    if (text) {
      const lumFg = lumOf(color);
      const lumBg = lumOf(bgEffective);
      if (lumFg != null && lumBg != null) {
        const ratio = contrast(lumFg, lumBg);
        const large = item.fontSize >= 24 || (item.fontSize >= 18.66 && parseInt(item.fontWeight) >= 700);
        const aa = large ? ratio >= 3 : ratio >= 4.5;
        const aaa = large ? ratio >= 4.5 : ratio >= 7;
        item.contrast = { ratio: Math.round(ratio * 100) / 100, aa, aaa, large };
      }
    }
    inventory.push(item);
  }

  // ── Overflow / clipping ───────────────────────────────────────────────────
  // One seeded overflow ripples up the tree: the leaf element sticks out, so
  // every ancestor's scrollWidth also exceeds its clientWidth. Report the
  // page-level scroll and the leaf cause, and skip an element whose overflow
  // is just a flagged descendant's — otherwise one defect becomes a cascade.
  const overflow = [];
  const docEl = document.scrollingElement || document.documentElement;
  if (docEl.scrollWidth > docEl.clientWidth) {
    overflow.push({
      element: 'document',
      kind: 'horizontal-page-overflow',
      detail: `scrollWidth ${docEl.scrollWidth} > clientWidth ${docEl.clientWidth} at ${w}px viewport`,
    });
  }
  const internalCandidates = [];
  const beyondCandidates = [];
  for (const el of elements) {
    const cs = getComputedStyle(el);
    if (el.scrollWidth > el.clientWidth) {
      internalCandidates.push({
        el,
        detail: `scrollWidth ${el.scrollWidth} > clientWidth ${el.clientWidth} (overflow-x: ${cs.overflowX})`,
      });
    }
    const r = el.getBoundingClientRect();
    if (r.right > docEl.clientWidth + 1 || r.left < -1) {
      beyondCandidates.push({
        el,
        detail: `rect [${Math.round(r.left)}, ${Math.round(r.right)}] exceeds ${w}px viewport`,
      });
    }
  }
  // A candidate whose overflow is explained by a flagged descendant is skipped
  // — the descendant is the leaf cause and will be reported itself.
  const isCause = (cand) => !candidates.some(other =>
    other !== cand && cand.el.contains(other.el));
  const candidates = [...internalCandidates, ...beyondCandidates];
  for (const cand of candidates) {
    if (!isCause(cand)) continue;
    const kind = cand.detail.startsWith('scrollWidth') ? 'internal-horizontal-overflow' : 'beyond-viewport';
    overflow.push({ element: describeEl(cand.el), kind, detail: cand.detail });
  }

  // ── Alignment / shared grid lines + spacing scale ─────────────────────────
  // Shared-grid-line detection is sibling-based: for every element whose
  // visible block children share a parent, the median left edge is the grid
  // line; a child a few px off it (but not a deliberate full indent) is a
  // near-miss misalignment. This avoids the false positives of a global
  // left-edge cluster, which mixes unrelated indentation levels.
  const blockish = (el) => {
    const d = getComputedStyle(el).display;
    return d.startsWith('block') || d === 'flex' || d === 'grid';
  };

  const alignment = [];
  const spacingScale = [];
  for (const parent of elements) {
    const kids = [...parent.children]
      .filter(k => blockish(k) && isVisible(k))
      .map(k => {
        const r = k.getBoundingClientRect();
        return { el: k, left: Math.round(r.left * 10) / 10, top: Math.round(r.top * 10) / 10, height: r.height };
      });
    if (kids.length < 2) continue;
    // Median left edge = the shared grid line of this sibling group.
    const sorted = kids.map(k => k.left).sort((a, b) => a - b);
    const median = sorted[Math.floor(sorted.length / 2)];
    for (const k of kids) {
      const dist = Math.abs(k.left - median);
      if (dist > 2 && dist <= 10) {
        alignment.push({
          element: describeEl(k.el),
          kind: 'grid-line-near-miss',
          detail: `left ${k.left} is ${Math.round(dist * 10) / 10}px off the sibling grid line ${median}`,
        });
      }
    }
    // Spacing scale: vertical gaps between consecutive siblings in this group,
    // rounded to the nearest 2px and deduplicated across the page.
    const byTop = kids.slice().sort((a, b) => a.top - b.top);
    for (let i = 1; i < byTop.length; i++) {
      const gap = Math.round((byTop[i].top - (byTop[i - 1].top + byTop[i - 1].height)) / 2) * 2;
      if (gap > 0) spacingScale.push(gap);
    }
  }
  const uniqueScale = [...new Set(spacingScale)].sort((a, b) => a - b);

  // ── Fonts ─────────────────────────────────────────────────────────────────
  const fontInfo = {
    status: document.fonts.status,
    families: [],
    fallbacks: [],
  };
  const seen = new Set();
  for (const el of elements) {
    const cs = getComputedStyle(el);
    const fam = cs.fontFamily.split(',')[0].trim().replace(/^['"]|['"]$/g, '');
    if (!fam || seen.has(fam)) continue;
    seen.add(fam);
    const loaded = document.fonts.check(`16px "${fam}"`);
    fontInfo.families.push({ family: fam, loaded });
    if (!loaded) {
      fontInfo.fallbacks.push({ family: fam, element: describeEl(el) });
    }
  }

  // ── z-index / overlap anomalies ───────────────────────────────────────────
  const overlap = [];
  const sized = elements
    .map(el => ({ el, r: el.getBoundingClientRect(), z: parseInt(getComputedStyle(el).zIndex) || 0 }))
    .filter(o => o.r.width > 20 && o.r.height > 20);
  for (let i = 0; i < sized.length; i++) {
    for (let j = i + 1; j < sized.length; j++) {
      const a = sized[i], b = sized[j];
      const ax = Math.max(0, Math.min(a.r.right, b.r.right) - Math.max(a.r.left, b.r.left));
      const ay = Math.max(0, Math.min(a.r.bottom, b.r.bottom) - Math.max(a.r.top, b.r.top));
      if (ax <= 0 || ay <= 0) continue;
      const inter = ax * ay;
      const minArea = Math.min(a.r.width * a.r.height, b.r.width * b.r.height);
      if (minArea === 0 || inter / minArea < 0.5) continue;
      // Parent/child containment isn't an anomaly; unrelated overlap is.
      const related = a.el.contains(b.el) || b.el.contains(a.el);
      if (related) continue;
      overlap.push({
        a: describeEl(a.el), b: describeEl(b.el),
        zA: a.z, zB: b.z,
        detail: `rects overlap ${Math.round(inter / minArea * 100)}% of the smaller (z-index ${a.z} vs ${b.z})`,
      });
    }
  }

  return {
    viewport: { width: w, height: fullPage ? docEl.scrollHeight : window.innerHeight },
    devicePixelRatio: window.devicePixelRatio,
    documentSize: { scrollWidth: docEl.scrollWidth, scrollHeight: docEl.scrollHeight, clientWidth: docEl.clientWidth },
    inventory,
    overflow,
    alignment,
    spacingScale: uniqueScale,
    fonts: fontInfo,
    overlaps: overlap,
  };
}

// ── Report assembly ──────────────────────────────────────────────────────────

function findingsFrom(measure) {
  const findings = [];
  for (const item of measure.inventory) {
    if (item.contrast && !item.contrast.aa) {
      findings.push({
        severity: 'ERROR',
        kind: 'contrast',
        element: item.text ? `${item.tag} "${item.text.slice(0, 40)}"` : describeEl({ tag: item.tag, id: item.id }),
        detail: `contrast ${item.contrast.ratio}:1 — fails WCAG AA (${item.contrast.large ? 'large text needs 3:1' : 'needs 4.5:1'})`,
      });
    } else if (item.contrast && !item.contrast.aaa && item.contrast.aa) {
      findings.push({
        severity: 'WARN',
        kind: 'contrast-aaa',
        element: item.text ? `${item.tag} "${item.text.slice(0, 40)}"` : item.tag,
        detail: `contrast ${item.contrast.ratio}:1 — passes AA but fails AAA (${item.contrast.large ? 'needs 4.5:1' : 'needs 7:1'})`,
      });
    }
  }
  for (const o of measure.overflow) {
    findings.push({ severity: 'ERROR', kind: 'overflow', element: o.element, detail: `${o.kind}: ${o.detail}` });
  }
  for (const a of measure.alignment) {
    findings.push({ severity: 'WARN', kind: 'alignment', element: a.element, detail: `${a.kind}: ${a.detail}` });
  }
  for (const o of measure.overlaps) {
    findings.push({ severity: 'WARN', kind: 'overlap', element: `${o.a} × ${o.b}`, detail: o.detail });
  }
  return findings;
}

function renderMarkdown(url, measures, opts, prose) {
  const lines = [];
  lines.push(`# Render report: ${url}`);
  lines.push('');
  lines.push(`- captured: ${new Date().toISOString()}`);
  lines.push(`- widths: ${opts.widths.join(', ')}${opts.fullPage ? ' (full page)' : ''}`);
  lines.push('');
  if (prose && prose.length) {
    lines.push('## Vision prose (--describe)');
    lines.push('');
    for (const p of prose) {
      if (p.error) {
        lines.push(`*--describe unavailable: ${p.error}*`);
        lines.push('');
        continue;
      }
      lines.push(`*Model: ${p.model}. Raw output, unedited:*`);
      lines.push('');
      lines.push(p.prose.trim());
      lines.push('');
    }
  }
  const allFindings = measures.flatMap(m => findingsFrom(m));
  if (allFindings.length === 0) {
    lines.push('## Findings');
    lines.push('');
    lines.push('None. No contrast, overflow, alignment, or overlap issues detected.');
    lines.push('');
  } else {
    const bySev = (sev) => allFindings.filter(f => f.severity === sev);
    const errors = bySev('ERROR');
    const warns = bySev('WARN');
    if (errors.length) {
      lines.push('## Findings — errors');
      lines.push('');
      for (const f of errors) lines.push(`- **[${f.kind}]** ${f.element}: ${f.detail}`);
      lines.push('');
    }
    if (warns.length) {
      lines.push('## Findings — warnings');
      lines.push('');
      for (const f of warns) lines.push(`- **[${f.kind}]** ${f.element}: ${f.detail}`);
      lines.push('');
    }
  }
  for (const m of measures) {
    lines.push(`## Viewport ${m.viewport.width}px`);
    lines.push('');
    lines.push(`- viewport: ${m.viewport.width}×${m.viewport.height}, DPR ${m.devicePixelRatio}`);
    lines.push(`- document: ${m.documentSize.scrollWidth}×${m.documentSize.scrollHeight} (client ${m.documentSize.clientWidth}px)`);
    lines.push(`- fonts: ${m.fonts.status}${m.fonts.fallbacks.length ? ` — fallbacks: ${m.fonts.fallbacks.map(f => f.family).join(', ')}` : ''}`);
    lines.push(`- spacing scale: ${m.spacingScale.length ? m.spacingScale.join(', ') : 'n/a'}`);
    lines.push('');
    if (m.inventory.length) {
      lines.push('### Element inventory');
      lines.push('');
      lines.push('| element | text | rect | font | contrast |');
      lines.push('|---|---|---|---|---|');
      for (const it of m.inventory) {
        const name = it.id ? `${it.tag}#${it.id}` : it.tag + (it.classes ? `.${it.classes[0]}` : '');
        const text = it.text ? `"${it.text.slice(0, 30)}"` : (it.interactive ? '(interactive)' : '');
        const rect = `${Math.round(it.rect.x)},${Math.round(it.rect.y)} ${Math.round(it.rect.width)}×${Math.round(it.rect.height)}`;
        const font = `${it.fontSize}px ${it.fontWeight}`;
        const contrastStr = it.contrast ? `${it.contrast.ratio}:1 ${it.contrast.aa ? (it.contrast.aaa ? 'AA/AAA' : 'AA') : 'FAIL'}` : '—';
        lines.push(`| ${name} | ${text} | ${rect} | ${font} | ${contrastStr} |`);
      }
      lines.push('');
    }
    if (m.overlap && m.overlap.length) {
      lines.push('### Overlap anomalies');
      lines.push('');
      for (const o of m.overlap) lines.push(`- ${o.a} × ${o.b}: ${o.detail}`);
      lines.push('');
    }
  }
  return lines.join('\n');
}

// ── Vision prose (--describe) ────────────────────────────────────────────────

// Zhipu's OpenAI-compatible v4 chat-completions endpoint (see docs/PLAYWRIGHT.md).
const VISION_ENDPOINT = 'https://open.bigmodel.cn/api/paas/v4/chat/completions';
const VISION_MODEL_DEFAULT = 'glm-4v-flash';

// The design-review prompt: layout, hierarchy, typography, color/contrast,
// spacing, "what looks off". The model's raw output is folded into the report
// verbatim — artifact transparency for the human reviewer.
const VISION_PROMPT = `You are a senior product designer reviewing a rendered web page from a screenshot.
Describe what you see for a text-only reviewer who cannot view the image. Cover:
1. Layout: overall structure, columns, sections, how content is arranged.
2. Visual hierarchy: what draws the eye first, whether the primary action is obvious.
3. Typography: font sizes, weights, line heights, any obvious type issues.
4. Color and contrast: palette, any elements that look low-contrast or hard to read.
5. Spacing: whether spacing feels consistent or cramped/scattered.
6. What looks off: anything visually broken, misaligned, overflowing, or odd.
Be concrete and specific — name the elements and where they are.`;

// describePng sends one captured PNG to the vision model and returns the raw
// prose. Errors are explicit: no key, network failure, or a non-OK response
// (a 404 names the model so the fix is a one-line SHOT_VISION_MODEL change).
async function describePng(pngPath, url, width) {
  const key = process.env.ZHIPU_API_KEY;
  if (!key) {
    throw new Error('--describe needs ZHIPU_API_KEY in the environment (set it in manigot/.env; it is forwarded into zai-profile sessions).');
  }
  const model = process.env.SHOT_VISION_MODEL || VISION_MODEL_DEFAULT;
  const b64 = fs.readFileSync(pngPath).toString('base64');
  const body = {
    model,
    messages: [{
      role: 'user',
      content: [
        { type: 'image_url', image_url: { url: `data:image/png;base64,${b64}` } },
        { type: 'text', text: `${VISION_PROMPT}\n\nRendered URL: ${url} (viewport width ${width}px).` },
      ],
    }],
  };
  let res;
  try {
    res = await fetch(VISION_ENDPOINT, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${key}`, 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  } catch (err) {
    throw new Error(`--describe: network error calling ${VISION_ENDPOINT}: ${err.message}`);
  }
  if (!res.ok) {
    const detail = (await res.text().catch(() => '')).slice(0, 300);
    throw new Error(`--describe: vision API returned ${res.status} (model '${model}'). If the model name is wrong, set SHOT_VISION_MODEL. Response: ${detail}`);
  }
  const data = await res.json();
  const prose = data.choices && data.choices[0] && data.choices[0].message && data.choices[0].message.content;
  if (!prose) {
    throw new Error(`--describe: vision API response had no content: ${JSON.stringify(data).slice(0, 300)}`);
  }
  return { model, prose };
}

// ── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  if (opts.help) {
    console.log(usage());
    return;
  }
  guardAgent();

  const outDir = path.join(resolveOutDir(), 'screenshots');
  fs.mkdirSync(outDir, { recursive: true });
  const slug = urlSlug(opts.url);

  let browser;
  try {
    browser = await chromium.launch();
    const measures = [];
    const prose = [];
    for (const width of opts.widths) {
      const context = await browser.newContext({
        viewport: { width, height: DEFAULT_HEIGHT },
        deviceScaleFactor: 1,
      });
      const page = await context.newPage();
      await page.goto(opts.url, { waitUntil: 'networkidle', timeout: 30000 }).catch(async () => {
        // networkidle can hang on pages with long-polling; load is enough.
        await page.goto(opts.url, { waitUntil: 'load', timeout: 30000 });
      });
      const measure = await page.evaluate(measurePage, { width, fullPage: opts.fullPage });
      const pngPath = path.join(outDir, `${slug}-${width}.png`);
      await page.screenshot({ path: pngPath, fullPage: opts.fullPage });
      measures.push(measure);
      console.log(`shot: captured ${width}px -> ${path.relative(process.cwd(), pngPath)}`);
      // Describe the primary (first requested) capture — one prose block per
      // run, folded into the report's prose section verbatim. A failure is
      // recorded in the report, not fatal: the capture and measured report are
      // the primary layer and must still land.
      if (opts.describe && width === opts.widths[0]) {
        try {
          const desc = await describePng(pngPath, opts.url, width);
          prose.push(desc);
          console.log(`shot: vision prose (${desc.model}) -> ${desc.prose.length} chars`);
        } catch (err) {
          prose.push({ model: process.env.SHOT_VISION_MODEL || 'glm-4v-flash', prose: '', error: err.message });
          console.error(`shot: ${err.message}`);
        }
      }
      await context.close();
    }
    const md = renderMarkdown(opts.url, measures, opts, prose);
    const mdPath = path.join(outDir, 'render-report.md');
    const jsonPath = path.join(outDir, 'render-report.json');
    fs.writeFileSync(mdPath, md);
    fs.writeFileSync(jsonPath, JSON.stringify({ url: opts.url, capturedAt: new Date().toISOString(), options: { widths: opts.widths, fullPage: opts.fullPage, describe: opts.describe }, prose, measures }, null, 2));
    console.log(`shot: report -> ${path.relative(process.cwd(), mdPath)} (+ render-report.json)`);
    const errors = measures.flatMap(m => findingsFrom(m)).filter(f => f.severity === 'ERROR');
    if (errors.length) {
      console.log(`shot: ${errors.length} error finding(s):`);
      for (const e of errors) console.log(`  - [${e.kind}] ${e.element}: ${e.detail}`);
    }
  } catch (err) {
    console.error(`shot: ${err.message}`);
    process.exit(1);
  } finally {
    if (browser) await browser.close().catch(() => {});
  }
}

main();
