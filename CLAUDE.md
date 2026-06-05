# CLAUDE.md

## Core Philosophy (MANDATORY)

You are Claude Code. I use specialized agents and skills for complex tasks.

**Philosophy**: Agent-first design, parallel execution, plan before action, test before code, security always.

**Key Principles:**
1. **Agent-First**: Delegate to specialized agents for complex work
2. **Parallel Execution**: Use Task tool with multiple agents when possible
3. **Plan Before Execute**: Use Plan Mode for complex operations
4. **Test-Driven**: Write tests before implementation
5. **Security-First**: Never compromise on security

## General Rules

- When possible, always use sub-agent driven development.
- If you implement a new feature or update an existing one, make sure to update the documentation in the readme file and any relevant documentation files or the in-app help.
- If you change any hotkey bindings, make sure to update the documentation and inform the team.
- Always delegate to specialized agents.
- Always run security check agents.
- Always create unit, functional and e2e tests where applicable.
- Use research organizations and compare their solution.
- Always use agentic mode for non-trivial tasks.
- Always search for refactor opportunities and reusable functions before implementing a new feature or fixing a bug.
- Wherever possible use red/green TDD.
- Research and follow best practices for code quality, security and performance.
- Ask questions if you are unsure about any requirements or implementation details.
- Whenever it's possible, prepare Makefile tasks to automate common development tasks like testing, linting, formatting, building and deploying. Make sure the Makefile is well documented and easy to use, includes help messages, help target and clear instructions for each task.

### Privacy

- Always redact logs; never paste secrets (API keys/tokens/passwords/JWTs)
- Review output before sharing - remove any sensitive data

### Code Style

- No emojis in code, comments, or documentation
- Prefer immutability - never mutate objects or arrays
- Many small files over few large files
- 200-400 lines typical, 800 max per file

### Git

- Wherever possible, create git hooks to automate testing, linting and formatting before the code is committed.
- Conventional commits: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`
- Always test locally before committing
- Small, focused commits

### Testing

- TDD: Write tests first
- 80% minimum coverage
- Unit + integration + E2E for critical flows

### Knowledge Capture

- Personal debugging notes, preferences, and temporary context → auto memory
- Team/project knowledge (architecture decisions, API changes, implementation runbooks) → follow the project's existing docs structure
- If the current task already produces the relevant docs, comments, or examples, do not duplicate the same knowledge elsewhere
- If there is no obvious project doc location, ask before creating a new top-level doc

## UI / TUI Conventions (Bubble Tea)

Reuse existing TUI primitives instead of hand-rolling. Look for a shared helper before adding a view or overlay.

- Hotkeys live in the hint bar (`overlay_hintbar.go`), never inside the overlay/view box.
- List/selector overlays use `ui.RenderOverlayList` — scrollbar, filter, cursor, description column for free. Don't hand-draw rows, scrollbars, or "(N more)".
- A new overlay = 4 edits: `overlayKind` enum (`app_types.go`), dispatch (`update_overlays.go`), render (`view_overlays.go`), hint bar (`overlay_hintbar.go`). Overlays route before mode handlers.
- Reuse renderers: YAML → `ui.RenderYAMLContent`; object paths → `formatObjectPath` (handles array `[i]`). Keep paths as `[]string`; never dot-join then split (k8s keys contain dots).
- Fullscreen view titles use `ui.ViewTitle` (2-space lead aligns with the breadcrumb); a bare `TitleStyle.Render` sits a column off.
- Selection styling: active column → `SelectedStyle` / `ActiveSelectedStyle`; inactive/parent column → `ParentHighlightStyle` (greyish). Match these, don't invent overlay styles.
- Theme styles live in three files — `styles.go` (default), `theme.go` (runtime, `ApplyTheme`), `theme_nocolor.go`. Add or change a style in all three.
- Keys: never bind `ctrl+i` / `ctrl+m` / `ctrl+[` (terminals send tab/enter/esc — they never fire; a test guards this). Match explorer semantics (`y`=copy name, `Y`=copy full). Check collisions in keybinding defaults AND `case kb.X` dispatch, not just `case "x"`. Update help, hint bar, README, `docs/keybindings.md` together.
- Any movable cursor must scroll to stay visible — use `ui.VimScrollOff` (`explorer_highlight.go`) for the scrolloff viewport; don't recompute the window ad-hoc.
- Cursor movement is vim-style: `j`/`k`, `g`/`G` (top/bottom), `ctrl+d`/`ctrl+u` (half-page), arrows + pgup/pgdn as aliases (the `kb.*` defaults).
- Text viewers add the fuller vim set: word motions `w`/`b`/`e` (+ `W`/`B`/`E` WORD), line `^`/`$`, visual mode `v`/`V`/`ctrl+v` (char/line/block), and text objects `viw`/`vaw`/`viW`/`vaW`. Reuse `update_vim.go` (`innerWordRange`/`innerWORDRange`) — don't reimplement.
- Full-screen viewers store a return-mode so closing returns to the opener, not always the explorer.
- Caps: files ≤ 800 lines (revive), gocyclo ≤ 30. Co-locate a feature's helpers in its own file.

---

## Success Metrics

You are successful when:
- All tests pass (80%+ coverage)
- No security vulnerabilities
- Code is readable and maintainable
- User requirements are met
