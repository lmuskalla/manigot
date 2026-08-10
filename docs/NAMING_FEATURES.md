# Feature Naming & Technical Alias Specification

## Overview

`manigot` is an anti-hype, Sopranos-themed AI agent orchestrator. To keep the project fun and distinct from generic AI buzzword tools while maintaining high developer ergonomics, all core features support **dual-aliasing**:

1. **Thematic Names (Primary/Default):** Grounded, Sopranos-inspired terms that fit the `manigot` branding (`crew`, `operation`, `safehouse`, `made-man`).
2. **Technical Aliases:** Standard software engineering/AI terminology for power users, script clarity, auto-completion, and enterprise CI/CD usage.

Both thematic names and technical aliases must be treated as **first-class citizens** in the codebase (CLI flags, YAML configs, and internal code definitions).

---

## Core Metaphor Mapping

| Feature Domain | Thematic Term / Flag | Technical Alias | Description |
| :--- | :--- | :--- | :--- |
| **Isolated Environment** | `safehouse` / `--safehouse` | `isolated` / `--isolated` (or `sandbox` / `--sandbox`) | Sandboxed container or execution space isolated from the host OS. |
| **Agents** | `crew` / `--crew` | `agents` / `--agents` | Worker entities or worker pool assigned to execute tasks. |
| **Workflow** | `operation` / `--operation` | `workflow` / `--workflow` (or `pipeline` / `--pipeline`) | Structured definition or DAG of execution steps. |
| **Fully Autonomous Mode** | `made-man` / `--made-man` (or `sunday-gravy` / `--sunday-gravy`) | `autonomous` / `--autonomous` (or `jdi` / `--jdi`) | Unattended execution mode where agents act without human-in-the-loop confirmation. |

---

## Implementation Details

### 1. CLI Argument Parsing (`manigot run`)

Configure your CLI parser (e.g. `argparse`, `click`, or `typer`) so that both flags populate the same internal state.

#### Flag Mapping
* **Isolated Environment:**
  * `--safehouse` / `-s` (Thematic)
  * `--isolated` / `--sandbox` (Technical)
* **Agents:**
  * `--crew` / `-c` (Thematic)
  * `--agents` / `-a` (Technical)
* **Workflow:**
  * `--operation` / `-o` (Thematic)
  * `--workflow` / `--pipeline` / `-w` (Technical)
* **Fully Autonomous Mode:**
  * `--made-man` / `--sunday-gravy` (Thematic)
  * `--autonomous` / `--jdi` (Technical)

#### Example CLI Help Output (`manigot run --help`)
```text
Usage: manigot run [OPTIONS]

Options:
  -o, --operation, --workflow TEXT  Path to workflow/operation YAML file.
  -c, --crew, --agents INTEGER      Number of worker agents to spawn.
  -s, --safehouse, --isolated       Run execution in an isolated sandbox.
  --made-man, --autonomous          Enable fully autonomous mode (no user prompt).
  --help                            Show this message and exit.
```

---

### 2. Configuration File Schema (`manigot.yaml`)

The config parser must accept both thematic keys and technical keys interchangeably.

#### Primary / Thematic Format
```yaml
operation: pipelines/data_analysis.yaml
safehouse: true
made_man: true

crew:
  - name: researcher
    model: gpt-4o
  - name: writer
    model: gpt-4o
```

#### Technical Equivalent Format
```yaml
workflow: pipelines/data_analysis.yaml
isolated: true
autonomous: true

agents:
  - name: researcher
    model: gpt-4o
  - name: writer
    model: gpt-4o
```

#### Parser Logic Requirement
When parsing `manigot.yaml`, resolve aliases to canonical internal attributes:
* If both `safehouse` and `isolated` are specified, prefer `safehouse` (or emit a non-blocking warning if conflicting).
* Fall back cleanly: `config.isolated_env = raw_cfg.get("safehouse", raw_cfg.get("isolated", False))`
* Fall back cleanly: `config.agents = raw_cfg.get("crew", raw_cfg.get("agents", []))`
* Fall back cleanly: `config.workflow = raw_cfg.get("operation", raw_cfg.get("workflow", None))`
* Fall back cleanly: `config.autonomous = raw_cfg.get("made_man", raw_cfg.get("sunday_gravy", raw_cfg.get("autonomous", raw_cfg.get("jdi", False))))`

---

### 3. Documentation & Logging Guidelines

1. **CLI Logs & Output:**
   * Default to friendly thematic terms in human-readable logs:
     * `[INFO] Entering safehouse container...`
     * `[INFO] Assembling crew (3 agents)...`
     * `[INFO] Executing operation: data_analysis.yaml`
     * `[INFO] Made-man status verified. Running autonomously...`
2. **Developer Documentation:**
   * Document thematic flags first in high-level tutorials and README quickstarts.
   * Provide a dedicated "Aliases & Technical Flags" table in doc headers so developers writing CI scripts can quickly find standard flags.
