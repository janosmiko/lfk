# Changelog

## [0.9.35](https://github.com/janosmiko/lfk/compare/v0.9.34...v0.9.35) (2026-04-29)


### Features

* **app:** tackle PTY pain points from [#81](https://github.com/janosmiko/lfk/issues/81) — selection, mux mode, scrollback ([32be754](https://github.com/janosmiko/lfk/commit/32be7546df5bf3df871e7a9d9d38a45e912452b1))


### Bug Fixes

* **app:** require typed confirmation for action-menu Force Delete ([8b1b2b7](https://github.com/janosmiko/lfk/commit/8b1b2b768f37fc23d3bf0fab1143fe11b7e15a81)), closes [#89](https://github.com/janosmiko/lfk/issues/89)
* **app:** unify Force Delete help text across menus, dialogs, and docs ([41b3aba](https://github.com/janosmiko/lfk/commit/41b3abac191aa8ad7cc50b1efe37e83257815219))

## [0.9.34](https://github.com/janosmiko/lfk/compare/v0.9.33...v0.9.34) (2026-04-29)


### Features

* **app:** add read-only mode with per-context [RO] markers ([1b1d9c1](https://github.com/janosmiko/lfk/commit/1b1d9c1738db93ea1b82f9979e8eaef51764832a))
* **app:** add read-only mode with per-context [RO] markers ([c148097](https://github.com/janosmiko/lfk/commit/c148097833a0771762632562b5b9066c696f7f6d))
* **app:** apply y/Y to multi-selection ([ce71b97](https://github.com/janosmiko/lfk/commit/ce71b97e6acba73d8ffc4cd35a1f669212735925))
* **app:** route :export through the Y bulk dispatcher ([d550328](https://github.com/janosmiko/lfk/commit/d55032814bfed5d718126e1624bc59ee6929487d))
* **ui:** add y to copy cursor row from rollback / history overlays ([5cc7cf6](https://github.com/janosmiko/lfk/commit/5cc7cf64009ac20f9e826d2373274479c5f717d9))


### Bug Fixes

* **app:** apply Y bulk to LevelOwned and skip false bulk at LevelContainers ([e366579](https://github.com/janosmiko/lfk/commit/e366579924f2d286702f7d6d0c9bf49f2826e949))
* **app:** plug read-only bypasses across labels and overlays ([66a11d0](https://github.com/janosmiko/lfk/commit/66a11d0f421d9cde78b874e04402299b373df3f2))
* **ui:** gate :sort command and column-header clicks on sortApplies() ([f8a7941](https://github.com/janosmiko/lfk/commit/f8a794135bd5f6f8a77ae7a109aad6b862e3a9db))
* **ui:** hide no-op sort and actions at picker levels ([7376449](https://github.com/janosmiko/lfk/commit/7376449eb354be3449f3080a1ef1f65120f3abb6))

## [0.9.33](https://github.com/janosmiko/lfk/compare/v0.9.32...v0.9.33) (2026-04-28)


### Bug Fixes

* **ui:** show full hotkey hint bar with log preview on ([#71](https://github.com/janosmiko/lfk/issues/71)) ([0badd03](https://github.com/janosmiko/lfk/commit/0badd0302d56738c7c6934fc38b3c4f457ac4e83))


### Performance Improvements

* **discovery:** persist API discovery to disk for stale-while-revalidate startup ([a1aaf27](https://github.com/janosmiko/lfk/commit/a1aaf27622f1809ab7bef9214f3cff19e3235399))
