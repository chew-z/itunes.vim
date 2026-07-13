# Documentation

Developer documentation for the iTunes/Apple Music CLI & MCP server. The
[README](../README.md) covers installation and usage; these documents cover
how the system works inside.

- **[ARCHITECTURE.md](ARCHITECTURE.md)** — components, the two data paths
  (database search vs JXA control), the `itunes.Client` API surface, error
  model, context propagation, and design invariants.
- **[DATABASE.md](DATABASE.md)** — SQLite schema (v6), FTS5 search internals,
  migration history, and the refresh/repopulation pipeline.
- **[JXA-AUTOMATION.md](JXA-AUTOMATION.md)** — how embedded JXA scripts are
  executed, the script inventory, the bulk-fetch rule (read before writing any
  JXA), and macOS platform limitations.
- **[VIM-PLUGIN.md](VIM-PLUGIN.md)** — the Vim/Neovim frontend in
  `vim-plugin/`: how it shells out to the CLI, fzf integration design, and
  the CLI contract it depends on.

`CLAUDE.md` at the repo root is AI-assistant guidance and duplicates some of
this on purpose; when they disagree, these docs plus the code are canonical.
