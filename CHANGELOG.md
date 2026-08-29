# Changelog

## [0.18.5](https://github.com/janosmiko/lfk/compare/v0.18.4...v0.18.5) (2026-08-29)


### Features

* probe common URL prefixes when reaching Prometheus ([#710](https://github.com/janosmiko/lfk/issues/710)) ([06c7eef](https://github.com/janosmiko/lfk/commit/06c7eefbfe916bdcbfbca19533ba3458161fce7b))


### Bug Fixes

* correctly identify cordoned nodes using spec.unschedulable ([#706](https://github.com/janosmiko/lfk/issues/706)) ([7173221](https://github.com/janosmiko/lfk/commit/7173221bf7de77acf68a9602472ae1e4a357e944))
* list every probed Prometheus target in the proxy error ([#709](https://github.com/janosmiko/lfk/issues/709)) ([288eb27](https://github.com/janosmiko/lfk/commit/288eb2752f362acd4fef108ca348b1624859aaee))

## [0.18.4](https://github.com/janosmiko/lfk/compare/v0.18.3...v0.18.4) (2026-08-28)


### Features

* discover victoriametrics monitoring endpoints ([#702](https://github.com/janosmiko/lfk/issues/702)) ([7b79b5a](https://github.com/janosmiko/lfk/commit/7b79b5a2c5620dd5eaac1cd7bd9206b023c9017f))

## [0.18.3](https://github.com/janosmiko/lfk/compare/v0.18.2...v0.18.3) (2026-08-26)


### Features

* show cpu and memory history as sparklines in the resource columns ([#698](https://github.com/janosmiko/lfk/issues/698)) ([610e341](https://github.com/janosmiko/lfk/commit/610e34183df6d1d70f8ca6b6ff9f8cbc032fb5ce))


### Bug Fixes

* scope multi-select keys by kind ([#697](https://github.com/janosmiko/lfk/issues/697)) ([db1192f](https://github.com/janosmiko/lfk/commit/db1192f4c6eb955110b26b2e1c5432ef2183cf87))
* show the cluster dashboard as one frame, and load it faster ([#695](https://github.com/janosmiko/lfk/issues/695)) ([bbf7c68](https://github.com/janosmiko/lfk/commit/bbf7c685e0557c635f8e734295a8539f1a1e3401))
* survive a proxy that resets long-lived watch connections ([#701](https://github.com/janosmiko/lfk/issues/701)) ([62ab9fe](https://github.com/janosmiko/lfk/commit/62ab9fe9ae931994bba86839f2ba2a6f6af497a2))
* use interactive bash when available ([#700](https://github.com/janosmiko/lfk/issues/700)) ([59ed623](https://github.com/janosmiko/lfk/commit/59ed62382ba40d7b0a9526d91da35ab9e97840ca))

## [0.18.2](https://github.com/janosmiko/lfk/compare/v0.18.1...v0.18.2) (2026-08-25)


### Features

* remember viewer display toggles across restarts ([#690](https://github.com/janosmiko/lfk/issues/690)) ([2bbc6b4](https://github.com/janosmiko/lfk/commit/2bbc6b4fb9ad3f8c19c9bd1b4d440bde8a879c64))


### Bug Fixes

* cluster-scoped views ignore the namespace selection ([#677](https://github.com/janosmiko/lfk/issues/677)) ([d7651e7](https://github.com/janosmiko/lfk/commit/d7651e7371d99abbc1bbfc185840992c0dd1ea83))
* lock the load-modify-save in column prefs and sort memory ([#693](https://github.com/janosmiko/lfk/issues/693)) ([68113b6](https://github.com/janosmiko/lfk/commit/68113b63b13711f08517c6f80dc1f92a6e012101))
* respect the kubeconfig context namespace at startup ([#688](https://github.com/janosmiko/lfk/issues/688)) ([dab5baf](https://github.com/janosmiko/lfk/commit/dab5baf6012813ff21019c50ca56ecccb9066f29))

## [0.18.1](https://github.com/janosmiko/lfk/compare/v0.18.0...v0.18.1) (2026-08-22)


### Bug Fixes

* cut idle CPU on the cluster dashboard ([#669](https://github.com/janosmiko/lfk/issues/669)) ([b2ef440](https://github.com/janosmiko/lfk/commit/b2ef44069f9e211b609addeadcbd879ee6df16fc))
* let two tests render under t.Parallel without racing ([#671](https://github.com/janosmiko/lfk/issues/671)) ([251b36f](https://github.com/janosmiko/lfk/commit/251b36fe6e287e7a0300badd01b6b5ec79f6d858))

## [0.18.0](https://github.com/janosmiko/lfk/compare/v0.17.5...v0.18.0) (2026-08-21)


### Features

* require Go 1.27.0 ([#662](https://github.com/janosmiko/lfk/issues/662)) ([4cfd6e1](https://github.com/janosmiko/lfk/commit/4cfd6e17f2cb10941c3c55f2a0b3b6658ace02ff))


### Bug Fixes

* bound local-cluster List calls and thread reqCtx into kubectl apply ([#650](https://github.com/janosmiko/lfk/issues/650)) ([b54be9f](https://github.com/janosmiko/lfk/commit/b54be9f4fcdcb05842df50984e6dfc9a2b691c64))
* cancel superseded update listeners in the port-forward and capture managers ([#664](https://github.com/janosmiko/lfk/issues/664)) ([ab1e7b4](https://github.com/janosmiko/lfk/commit/ab1e7b494a77d0d5e50ca9b32fc9e2e19fbf15e0))
* close exec PTYs on shutdown and evict dead port-forward entries ([#654](https://github.com/janosmiko/lfk/issues/654)) ([749ae45](https://github.com/janosmiko/lfk/commit/749ae45c54b2cefa058dab94a75afefe39d1d208))
* harden the log redactor against structured secret shapes ([#658](https://github.com/janosmiko/lfk/issues/658)) ([5aa651e](https://github.com/janosmiko/lfk/commit/5aa651e9f62e6736a1bebbd7fca3694c9aeb617a))
* keep the discovery-error fallback from clobbering a deeper level ([#667](https://github.com/janosmiko/lfk/issues/667)) ([9bd2ba9](https://github.com/janosmiko/lfk/commit/9bd2ba9b837ded430046f65d0ab429a741594e0f))
* log the pod sub-fetch error in the CronJob resource tree ([#653](https://github.com/janosmiko/lfk/issues/653)) ([5938ad7](https://github.com/janosmiko/lfk/commit/5938ad7dc5fbbaad3997da63efe8a11bbe0d1bbe))
* redact subprocess output before it reaches logs and the status bar ([#647](https://github.com/janosmiko/lfk/issues/647)) ([4b4b29b](https://github.com/janosmiko/lfk/commit/4b4b29b83468408e4bf3efbf1b1dc120f7d4348d))
* sanitize the exec terminal title before rendering ([#651](https://github.com/janosmiko/lfk/issues/651)) ([0fbc0ca](https://github.com/janosmiko/lfk/commit/0fbc0ca5d45486a8b1f5154a0500db1dc63899d7))
* score fuzzy-filtered items on the field that matched them ([#668](https://github.com/janosmiko/lfk/issues/668)) ([5889618](https://github.com/janosmiko/lfk/commit/588961896baac333767ac2763dfc437673ecc7b2))


### Performance Improvements

* cache the per-frame recomputation in the explorer render path ([#655](https://github.com/janosmiko/lfk/issues/655)) ([d60cddd](https://github.com/janosmiko/lfk/commit/d60cdddf1fb4ff7d0fa7f4950910fe6457452655))
* serve watch-tick refreshes from the informer cache ([#659](https://github.com/janosmiko/lfk/issues/659)) ([de7a51d](https://github.com/janosmiko/lfk/commit/de7a51d105a31335ae19b409fe002bc9b1464227))
* skip preview refetch when the hovered object is unchanged ([#656](https://github.com/janosmiko/lfk/issues/656)) ([612da6f](https://github.com/janosmiko/lfk/commit/612da6f926ef7fb3c7b2890c519ae45308eaa1dd))
* throttle list metrics refresh to a minimum interval ([#661](https://github.com/janosmiko/lfk/issues/661)) ([4abb7a6](https://github.com/janosmiko/lfk/commit/4abb7a6b313ab206d7bbf6e719c4e9fe212cbde4))

## [0.17.5](https://github.com/janosmiko/lfk/compare/v0.17.4...v0.17.5) (2026-08-19)


### Bug Fixes

* sort relative-time columns by duration ([#643](https://github.com/janosmiko/lfk/issues/643)) ([49013f8](https://github.com/janosmiko/lfk/commit/49013f834d0ceb0ad6d372cf22cd85e3afa27e9c))

## [0.17.4](https://github.com/janosmiko/lfk/compare/v0.17.3...v0.17.4) (2026-08-18)


### Bug Fixes

* sort the Changed and Uptime columns by their real timestamps ([#641](https://github.com/janosmiko/lfk/issues/641)) ([54b38b3](https://github.com/janosmiko/lfk/commit/54b38b3fc27de944234f970511e04c3850044c0e))

## [0.17.3](https://github.com/janosmiko/lfk/compare/v0.17.2...v0.17.3) (2026-08-17)


### Features

* add a Changed column with the time since the last change ([#631](https://github.com/janosmiko/lfk/issues/631)) ([0b8d926](https://github.com/janosmiko/lfk/commit/0b8d926b4039937af81a3c35f08e95bb3f59d4c3))
* **nav:** make the 0/1/2 level keys walk back down ([#633](https://github.com/janosmiko/lfk/issues/633)) ([63b5333](https://github.com/janosmiko/lfk/commit/63b5333323f8f76f057c6d623db617f115500b9e))

## [0.17.2](https://github.com/janosmiko/lfk/compare/v0.17.1...v0.17.2) (2026-08-15)


### Bug Fixes

* **test:** make the suite pass the way packagers build it ([#628](https://github.com/janosmiko/lfk/issues/628)) ([4c49389](https://github.com/janosmiko/lfk/commit/4c493891745dda0f71b1515de20efeedfd8c8e87))

## [0.17.1](https://github.com/janosmiko/lfk/compare/v0.17.0...v0.17.1) (2026-08-14)


### Features

* **logviewer:** add config setting for log wrap ([#624](https://github.com/janosmiko/lfk/issues/624)) ([7b15e2c](https://github.com/janosmiko/lfk/commit/7b15e2c6e32d36873bb4d5cb25414e86c4cf59a6))

## [0.17.0](https://github.com/janosmiko/lfk/compare/v0.16.1...v0.17.0) (2026-08-12)


### ⚠ BREAKING CHANGES

* action-menu keys changed. Export Template is "T"; Log Top, Terminate Sync and Terminate Workflow move from "T" to "t". Log Top is no longer offered on Jobs or CronJobs.

### Features

* add a built-in demo cluster behind --demo (TASK-865) ([#594](https://github.com/janosmiko/lfk/issues/594)) ([63793e5](https://github.com/janosmiko/lfk/commit/63793e53d2f7ac34b4ae5c60d00297602e331185))
* add a tainted-string type for cluster-sourced fields (TASK-874) ([#610](https://github.com/janosmiko/lfk/issues/610)) ([4598a69](https://github.com/janosmiko/lfk/commit/4598a69f6123cfa8fc9f5795d408a113b47c20bf))
* add an Undeliverable view for resources stuck waiting (TASK-855) ([#609](https://github.com/janosmiko/lfk/issues/609)) ([6688498](https://github.com/janosmiko/lfk/commit/6688498ff79d7db4e6d2008a468e2378e6a8c791))
* delete saved user templates from the template picker ([#619](https://github.com/janosmiko/lfk/issues/619)) ([6333f47](https://github.com/janosmiko/lfk/commit/6333f47dd0fbb17c939077ca441361a38784bb62))
* export a live resource as a reusable template (TASK-866) ([#618](https://github.com/janosmiko/lfk/issues/618)) ([3bae421](https://github.com/janosmiko/lfk/commit/3bae42198137f630dd562dc7a6fa0d656a8c1742))
* hide actions the current user is not allowed to run (TASK-872) ([#600](https://github.com/janosmiko/lfk/issues/600)) ([347e39e](https://github.com/janosmiko/lfk/commit/347e39e2e9c9d450a42e78bbdebcd96f688def0d))
* name the person on every lfk write ([#586](https://github.com/janosmiko/lfk/issues/586)) ([5230859](https://github.com/janosmiko/lfk/commit/5230859decf190c67a405e2d375af1b85a1a37a6))
* show field-manager blame in the YAML viewer ([#584](https://github.com/janosmiko/lfk/issues/584)) ([8fffe94](https://github.com/janosmiko/lfk/commit/8fffe9499b11fe5cace68937c20691af1d6cc388))
* show the blast radius in destructive confirm dialogs ([#589](https://github.com/janosmiko/lfk/issues/589)) ([2e2a875](https://github.com/janosmiko/lfk/commit/2e2a87542796c775cfdb93d33265407fc51649a2))
* show the cluster schema description of the field under the cursor ([#588](https://github.com/janosmiko/lfk/issues/588)) ([b0fbbbf](https://github.com/janosmiko/lfk/commit/b0fbbbf0353553bc6ef8427a79e3951fe3c4be58))
* state the cost of a destructive action as Scope, Availability and Risk (TASK-862, TASK-877) ([#599](https://github.com/janosmiko/lfk/issues/599)) ([8349246](https://github.com/janosmiko/lfk/commit/8349246e37bf62f5fccf8d5328aff74d83752194))


### Bug Fixes

* bound and redact the kubectl explain subprocesses (TASK-869) ([#596](https://github.com/janosmiko/lfk/issues/596)) ([f57406e](https://github.com/janosmiko/lfk/commit/f57406e6880b8b80ffb8920652611bef3d6bcdfc))
* correct orphan overview keybinding from Shift+O to Shift+Z in docs ([#611](https://github.com/janosmiko/lfk/issues/611)) ([d2d0b4e](https://github.com/janosmiko/lfk/commit/d2d0b4eae748de28b5e693198156caecac2e64e4))
* drop API Explorer replies from an older session (TASK-876) ([#601](https://github.com/janosmiko/lfk/issues/601)) ([098c1b8](https://github.com/janosmiko/lfk/commit/098c1b8c7cb2191a74f0d320a61d239fa860f869))
* fail loudly on a marshalled tainted.String, drop dead event fields (TASK-887) ([#615](https://github.com/janosmiko/lfk/issues/615)) ([efbba3e](https://github.com/janosmiko/lfk/commit/efbba3e3a0ac7152019a716e3a6c7fb8aa2761f0))
* key saved templates by namespace, not just name ([#621](https://github.com/janosmiko/lfk/issues/621)) ([fa80fcd](https://github.com/janosmiko/lfk/commit/fa80fcd602b8bb39e44edafc02e7b6cda9a55e36))
* measure emoji icons the way the terminal does ([#604](https://github.com/janosmiko/lfk/issues/604)) ([#605](https://github.com/janosmiko/lfk/issues/605)) ([6b9a83b](https://github.com/janosmiko/lfk/commit/6b9a83b48cc0a0b4e1f4c0ab9607505f45b2752e))
* measure wrapCrashText by display width (TASK-882) ([#607](https://github.com/janosmiko/lfk/issues/607)) ([3f4d614](https://github.com/janosmiko/lfk/commit/3f4d61454a4d647ebd668533de496e13ddb515df))
* open a restored session on its saved view ([#593](https://github.com/janosmiko/lfk/issues/593)) ([9554cf7](https://github.com/janosmiko/lfk/commit/9554cf7a37f3c8452979413107a5df49b21ca907))
* report a status message when every multi-log stream fails to start ([#622](https://github.com/janosmiko/lfk/issues/622)) ([34c562a](https://github.com/janosmiko/lfk/commit/34c562ad547e577aba9b8041a5b950dfa6a8ca7a))
* resolve CronJob logs through its newest Job's pods ([#620](https://github.com/janosmiko/lfk/issues/620)) ([de31d3a](https://github.com/janosmiko/lfk/commit/de31d3a30a55a6d23705a57da3a02ecfa29e5630))
* resume the API Explorer schema load on tab restore (TASK-878) ([#602](https://github.com/janosmiko/lfk/issues/602)) ([5d55a3f](https://github.com/janosmiko/lfk/commit/5d55a3f1f82db1cb5944e1ebc45b3649d3893eb8))
* resume the API Explorer's pending fetch, not its last level (TASK-879) ([#608](https://github.com/janosmiko/lfk/issues/608)) ([537a844](https://github.com/janosmiko/lfk/commit/537a8449f48b25df3c40f39807a474166f06e526))
* route port-forward and apply argv through the demo helper (TASK-875) ([#603](https://github.com/janosmiko/lfk/issues/603)) ([59d9ca1](https://github.com/janosmiko/lfk/commit/59d9ca148cff1af0fde5f8905acf649f91fa6a8e))
* sanitize every status-bar message at its shared sink (TASK-884) ([#612](https://github.com/janosmiko/lfk/issues/612)) ([e44c5cd](https://github.com/janosmiko/lfk/commit/e44c5cd8bbd50d324e348a860cf1afeb0671a123))
* sanitize terminal escapes in describe, YAML, event and editor sinks (TASK-873) ([#595](https://github.com/janosmiko/lfk/issues/595)) ([a390fd2](https://github.com/janosmiko/lfk/commit/a390fd2a0bc6ab05b7e9cf7c2d31da72aed21965))
* sanitize the render sinks TASK-873 did not reach (TASK-880) ([#606](https://github.com/janosmiko/lfk/issues/606)) ([2f79f42](https://github.com/janosmiko/lfk/commit/2f79f422f7816db34fa1e5151bb60c001ecb86f1))
* stop the orphan scanner reporting a kind whose referencing lists failed (TASK-886) ([#614](https://github.com/janosmiko/lfk/issues/614)) ([aa19af3](https://github.com/janosmiko/lfk/commit/aa19af3827c0c969390b0f494147726846655f39))

## [0.16.1](https://github.com/janosmiko/lfk/compare/v0.16.0...v0.16.1) (2026-08-06)


### Bug Fixes

* apply dual colorscheme again under Bubble Tea v2 ([#580](https://github.com/janosmiko/lfk/issues/580)) ([#581](https://github.com/janosmiko/lfk/issues/581)) ([0a16c12](https://github.com/janosmiko/lfk/commit/0a16c1246a57eb4cd584bfd638c85cdd47f28dad))

## [0.16.0](https://github.com/janosmiko/lfk/compare/v0.15.16...v0.16.0) (2026-08-06)


### ⚠ BREAKING CHANGES

* "?" no longer opens the help screen from the explorer; use F1, or set which_key_leader to another key.
* the TUI now runs on Bubble Tea v2, which negotiates keyboard and mouse capabilities with the terminal differently from v1. Canonical keybinding spellings changed as listed above; configs using the old spellings are normalised automatically, but the stored and displayed form differs. The default mouse_toggle binding is now "ctrl+alt+y" and toggle_select is "space".

### Features

* context-aware which-key panel, and a reworked help screen ([#578](https://github.com/janosmiko/lfk/issues/578)) ([d8d77f3](https://github.com/janosmiko/lfk/commit/d8d77f3b7c703f140b16a2329c633bebb3791655))
* migrate to Bubble Tea v2 and support Ctrl+Shift keybindings ([#573](https://github.com/janosmiko/lfk/issues/573)) ([a5da319](https://github.com/janosmiko/lfk/commit/a5da3199b5058dceb611fc2a15f2d156075a4a03))


### Bug Fixes

* scope Ctrl+C bulk-action cancel to the tab that started it ([#579](https://github.com/janosmiko/lfk/issues/579)) ([4f5f774](https://github.com/janosmiko/lfk/commit/4f5f774fb52de4faee15052aa3becd7cee471c7e))

## [0.15.16](https://github.com/janosmiko/lfk/compare/v0.15.15...v0.15.16) (2026-07-31)


### Features

* **delete:** select the cascade policy for force delete too ([9ee809f](https://github.com/janosmiko/lfk/commit/9ee809f0ded0371025a9cb7ca1e601f5c37238ac))
* **delete:** selectable cascade policy for delete and force delete ([#570](https://github.com/janosmiko/lfk/issues/570)) ([9ee809f](https://github.com/janosmiko/lfk/commit/9ee809f0ded0371025a9cb7ca1e601f5c37238ac))

## [0.15.15](https://github.com/janosmiko/lfk/compare/v0.15.14...v0.15.15) (2026-07-27)


### Features

* **taints:** add a picker for widely used taints ([#565](https://github.com/janosmiko/lfk/issues/565)) ([1e3b7f1](https://github.com/janosmiko/lfk/commit/1e3b7f198a96c4bbbb2b9d06089b674bfad7d44a))


### Bug Fixes

* **actions:** target the correct API group when two kinds collide ([#566](https://github.com/janosmiko/lfk/issues/566)) ([5eddf5f](https://github.com/janosmiko/lfk/commit/5eddf5fa04ab5ac963f3bbeeffede398af82bc52)), closes [#562](https://github.com/janosmiko/lfk/issues/562)
* details preview without labels and fast terminal scrolling buttons ([#568](https://github.com/janosmiko/lfk/issues/568)) ([57b33da](https://github.com/janosmiko/lfk/commit/57b33da965963f83e62d7358f62e16d84c57d764))
* **sort:** derive Status order from the shared severity classifier ([#563](https://github.com/janosmiko/lfk/issues/563)) ([4a46cf6](https://github.com/janosmiko/lfk/commit/4a46cf670945409aac1b468ad54e2acb28c04eaf))
* **taints:** keep the effect field visible in the add-row ([#564](https://github.com/janosmiko/lfk/issues/564)) ([a12df04](https://github.com/janosmiko/lfk/commit/a12df04f190f9cad8de94f11f51fdf22af1807a8))

## [0.15.14](https://github.com/janosmiko/lfk/compare/v0.15.13...v0.15.14) (2026-07-23)


### Bug Fixes

* **release:** lint PR titles as conventional commits to prevent missed releases ([#555](https://github.com/janosmiko/lfk/issues/555)) ([0115088](https://github.com/janosmiko/lfk/commit/011508892f83b1e53982cc8cd907f3c22822d409))

## [0.15.13](https://github.com/janosmiko/lfk/compare/v0.15.12...v0.15.13) (2026-07-21)


### Bug Fixes

* **explorer:** keep focused row untinted in foreground row-status-tint ([#540](https://github.com/janosmiko/lfk/issues/540)) ([#552](https://github.com/janosmiko/lfk/issues/552)) ([bc702d0](https://github.com/janosmiko/lfk/commit/bc702d091627a8beea57588ceb51e35a18031160))

## [0.15.12](https://github.com/janosmiko/lfk/compare/v0.15.11...v0.15.12) (2026-07-20)


### Features

* **explorer:** make @ cycle Cluster/Monitoring dashboard from any level ([#547](https://github.com/janosmiko/lfk/issues/547)) ([43ad7a3](https://github.com/janosmiko/lfk/commit/43ad7a34f74bffaec9201e5222ccf83106db68c8))

## [0.15.11](https://github.com/janosmiko/lfk/compare/v0.15.10...v0.15.11) (2026-07-20)


### Bug Fixes

* **netpol:** match Cilium namespace-derived label selectors ([#545](https://github.com/janosmiko/lfk/issues/545)) ([97e3862](https://github.com/janosmiko/lfk/commit/97e3862a01d93ddbc56c28384e1646f1e5521c80)), closes [#544](https://github.com/janosmiko/lfk/issues/544)

## [0.15.10](https://github.com/janosmiko/lfk/compare/v0.15.9...v0.15.10) (2026-07-17)


### Features

* **explorer:** tint failed/pending rows by status ([#540](https://github.com/janosmiko/lfk/issues/540)) ([#543](https://github.com/janosmiko/lfk/issues/543)) ([ea872db](https://github.com/janosmiko/lfk/commit/ea872db0dcf9304e91fa1f2a471caba2a0210f7a))


### Bug Fixes

* **explorer:** clear quick filter when goto jumps between resource types ([#541](https://github.com/janosmiko/lfk/issues/541)) ([4222cbf](https://github.com/janosmiko/lfk/commit/4222cbfcb25c7ddebdcaea3f5da078031612ea30))

## [0.15.9](https://github.com/janosmiko/lfk/compare/v0.15.8...v0.15.9) (2026-07-16)


### Bug Fixes

* **dashboard:** stop trackpad momentum from scrolling the wrong list ([#524](https://github.com/janosmiko/lfk/issues/524)) ([#537](https://github.com/janosmiko/lfk/issues/537)) ([d70f7fd](https://github.com/janosmiko/lfk/commit/d70f7fdac4f6d1915814f80e55cf5da4661702c7))
* roll up generic CRD summaries by phase ([#536](https://github.com/janosmiko/lfk/issues/536)) ([#539](https://github.com/janosmiko/lfk/issues/539)) ([2f52f6c](https://github.com/janosmiko/lfk/commit/2f52f6ce6c05965ba8af3ffd65ba5ce22b53a2c8))

## [0.15.8](https://github.com/janosmiko/lfk/compare/v0.15.7...v0.15.8) (2026-07-14)


### Bug Fixes

* **dashboard:** scroll the fullscreen dashboard with the mouse wheel ([#524](https://github.com/janosmiko/lfk/issues/524)) ([#534](https://github.com/janosmiko/lfk/issues/534)) ([654d4b4](https://github.com/janosmiko/lfk/commit/654d4b4bbff7e06d31628cf97f03b6cb4ec8312a))

## [0.15.7](https://github.com/janosmiko/lfk/compare/v0.15.6...v0.15.7) (2026-07-13)


### Features

* pinned dashboard summaries ([#525](https://github.com/janosmiko/lfk/issues/525)) ([#532](https://github.com/janosmiko/lfk/issues/532)) ([34b8ddf](https://github.com/janosmiko/lfk/commit/34b8ddf61eb93162e1b60c599c3a8d915e8fec43))

## [0.15.6](https://github.com/janosmiko/lfk/compare/v0.15.5...v0.15.6) (2026-07-10)


### Bug Fixes

* **explorer:** stop cursor-less renders from writing pane scroll state ([#524](https://github.com/janosmiko/lfk/issues/524)) ([#526](https://github.com/janosmiko/lfk/issues/526)) ([ca4a2e8](https://github.com/janosmiko/lfk/commit/ca4a2e8b5dac4b9a25b087a12a4c2bc7ffaa2d50))

## [0.15.5](https://github.com/janosmiko/lfk/compare/v0.15.4...v0.15.5) (2026-07-10)


### Bug Fixes

* **nodes:** describe drain in the drain confirmation, not delete ([#522](https://github.com/janosmiko/lfk/issues/522)) ([fa12b02](https://github.com/janosmiko/lfk/commit/fa12b02be93ecd43b4de4fb9fada27d09ebc6abc))

## [0.15.4](https://github.com/janosmiko/lfk/compare/v0.15.3...v0.15.4) (2026-07-10)


### Features

* **nodes:** show node uptime from Prometheus in the nodes list ([#520](https://github.com/janosmiko/lfk/issues/520)) ([d6f8e81](https://github.com/janosmiko/lfk/commit/d6f8e8154518e99bd8da50a63a4e7bc1ed846b0c))

## [0.15.3](https://github.com/janosmiko/lfk/compare/v0.15.2...v0.15.3) (2026-07-09)


### Bug Fixes

* **tabs:** isolate quick filter presets per tab ([#518](https://github.com/janosmiko/lfk/issues/518)) ([8dc64a0](https://github.com/janosmiko/lfk/commit/8dc64a01f916289553b2dc92dd988344017bd6e3))

## [0.15.2](https://github.com/janosmiko/lfk/compare/v0.15.1...v0.15.2) (2026-07-09)


### Features

* **bookmarks:** load saved namespace by default, show it in the overlay ([#517](https://github.com/janosmiko/lfk/issues/517)) ([d003cc0](https://github.com/janosmiko/lfk/commit/d003cc0fcca8e55c2cccb0b2ccc59f1a20a4f3d4))
* **namespace:** jump to previous namespace (g\) ([#515](https://github.com/janosmiko/lfk/issues/515)) ([ef40744](https://github.com/janosmiko/lfk/commit/ef4074472c03428de2c6bf6bfc16d6b8d444c8d7))


### Bug Fixes

* **nix:** bump Go override to 1.26.5 to match go.mod ([#514](https://github.com/janosmiko/lfk/issues/514)) ([39b68bb](https://github.com/janosmiko/lfk/commit/39b68bb8450c7873efb747f4a527a0e9c5f1f1b1))

## [0.15.1](https://github.com/janosmiko/lfk/compare/v0.15.0...v0.15.1) (2026-07-08)


### Features

* **actions:** add CronJob Suspend/Resume and unify toggle actions ([#505](https://github.com/janosmiko/lfk/issues/505)) ([0257a63](https://github.com/janosmiko/lfk/commit/0257a630412499699b6facc4abf6cbae3c18c498))
* **node:** stream kubectl drain output into the embedded terminal ([#506](https://github.com/janosmiko/lfk/issues/506)) ([5ef7591](https://github.com/janosmiko/lfk/commit/5ef75910827de4006281284d5927a537024b9db4))
* **sessions:** named sessions with an active-session model ([#509](https://github.com/janosmiko/lfk/issues/509)) ([d8da84b](https://github.com/janosmiko/lfk/commit/d8da84bf0078fa6d147f12cb5fd8b4bc179d431f))
* **tabs:** move active tab left/right ([#512](https://github.com/janosmiko/lfk/issues/512)) ([aadfac5](https://github.com/janosmiko/lfk/commit/aadfac529234170b6dbf35f06bae0a9e31c3eac1))


### Bug Fixes

* **namespace:** keep selector open on first Enter over a sole filtered result ([#508](https://github.com/janosmiko/lfk/issues/508)) ([ffbcdfb](https://github.com/janosmiko/lfk/commit/ffbcdfb0813679d16695bef15cc8bd3ee0e4a033))
* **node:** humanize node Mem Alloc (6895736Ki -&gt; 6.6Gi) ([#511](https://github.com/janosmiko/lfk/issues/511)) ([7c4ba0d](https://github.com/janosmiko/lfk/commit/7c4ba0d4401d8bff4c083b833cf96a11759b6b13))
* **sessions:** show live tab count for the active session ([#513](https://github.com/janosmiko/lfk/issues/513)) ([08b0c04](https://github.com/janosmiko/lfk/commit/08b0c04d0d581a737255340e759b7bb0ef6cf87f))

## [0.15.0](https://github.com/janosmiko/lfk/compare/v0.14.19...v0.15.0) (2026-07-07)


### ⚠ BREAKING CHANGES

* **kubeconfig:** when KUBECONFIG is set, lfk no longer merges ~/.kube/config and the default ~/.kube/config.d scan; set kubeconfig_exclusive: false to restore the old behavior.

### Features

* **kubeconfig:** make KUBECONFIG exclusive by default, with opt-out ([#491](https://github.com/janosmiko/lfk/issues/491)) ([744f497](https://github.com/janosmiko/lfk/commit/744f497dee56a86bc98e134bb581497acf48dcb1))
* node taint editor overlay (replaces Taint/Untaint prefill) ([#501](https://github.com/janosmiko/lfk/issues/501)) ([e77eac6](https://github.com/janosmiko/lfk/commit/e77eac6156423c7b1c5acf312359c78a8f44c4e7))
* single-field copy picker (ctrl+y) ([#500](https://github.com/janosmiko/lfk/issues/500)) ([36b4f98](https://github.com/janosmiko/lfk/commit/36b4f98cf0c9965283cb9c0ceda4e4e557b8deb1))


### Bug Fixes

* truncate overlay list rows to the item area ([#504](https://github.com/janosmiko/lfk/issues/504)) ([5b7acc4](https://github.com/janosmiko/lfk/commit/5b7acc43ebd2088d270486311a04634c05e927d9))

## [0.14.19](https://github.com/janosmiko/lfk/compare/v0.14.18...v0.14.19) (2026-07-02)


### Features

* **ui:** color printer-column values by status semantics ([#489](https://github.com/janosmiko/lfk/issues/489)) ([34d9de6](https://github.com/janosmiko/lfk/commit/34d9de675548f568d6cf7a101c04c7416100c167))

## [0.14.18](https://github.com/janosmiko/lfk/compare/v0.14.17...v0.14.18) (2026-06-25)


### Features

* **app:** throttle watch refresh when idle/unfocused + gate spinner ([#483](https://github.com/janosmiko/lfk/issues/483)) ([1b575b7](https://github.com/janosmiko/lfk/commit/1b575b7601c9d8c38276d0636fc6385d62368457))

## [0.14.17](https://github.com/janosmiko/lfk/compare/v0.14.16...v0.14.17) (2026-06-25)


### Features

* add quick filter presets for PersistentVolumes ([#482](https://github.com/janosmiko/lfk/issues/482)) ([93835fb](https://github.com/janosmiko/lfk/commit/93835fb9586a28c6dc6c027370c63859938d2241))


### Bug Fixes

* land cursor on jumped-to resource type after goto back-nav ([#480](https://github.com/janosmiko/lfk/issues/480)) ([0c91696](https://github.com/janosmiko/lfk/commit/0c91696bf75c90d238ab637d82984e4b646836d8))

## [0.14.16](https://github.com/janosmiko/lfk/compare/v0.14.15...v0.14.16) (2026-06-24)


### Features

* hide-left-sidebar toggle (|) for the explorer ([#476](https://github.com/janosmiko/lfk/issues/476)) ([c1a895a](https://github.com/janosmiko/lfk/commit/c1a895a12f4300fa8d52643759a15a19b24aebca))
* restore active list filter and cursor across restarts ([#477](https://github.com/janosmiko/lfk/issues/477)) ([273f230](https://github.com/janosmiko/lfk/commit/273f2307aa2aec354bdb58496c88a6f31c63e035))


### Bug Fixes

* handle discarded GetContexts errors in navigation ([#479](https://github.com/janosmiko/lfk/issues/479)) ([6f6145e](https://github.com/janosmiko/lfk/commit/6f6145e52be645a07ae9a291ad186d979fa82929))

## [0.14.15](https://github.com/janosmiko/lfk/compare/v0.14.14...v0.14.15) (2026-06-23)


### Features

* save and restore list filter in bookmarks ([#470](https://github.com/janosmiko/lfk/issues/470)) ([4927d59](https://github.com/janosmiko/lfk/commit/4927d59ec44f7e65251adbd66c7cadfc1b729831))

## [0.14.14](https://github.com/janosmiko/lfk/compare/v0.14.13...v0.14.14) (2026-06-22)


### Features

* goto navigation (g-prefix chords) + which-key overlay ([#469](https://github.com/janosmiko/lfk/issues/469)) ([f47054e](https://github.com/janosmiko/lfk/commit/f47054e49886bd26eecfb8b450eba1bf9d19e477))
* manual scaling for HorizontalPodAutoscalers ([#464](https://github.com/janosmiko/lfk/issues/464)) ([d2dbf9c](https://github.com/janosmiko/lfk/commit/d2dbf9cd020cd1f253aad4be7bf6ee13c644ced9))


### Bug Fixes

* **release:** make Chocolatey publish best-effort ([#460](https://github.com/janosmiko/lfk/issues/460)) ([82f6137](https://github.com/janosmiko/lfk/commit/82f61378b1920b995bbcb0754057a89e47540d7d))
* **ui:** AutoSync overlay polish — no highlight behind switches, drop in-overlay hint ([#468](https://github.com/janosmiko/lfk/issues/468)) ([c3cb486](https://github.com/janosmiko/lfk/commit/c3cb4861b6c514d91f576638cdf22ef3cb1f27a7))

## [0.14.13](https://github.com/janosmiko/lfk/compare/v0.14.12...v0.14.13) (2026-06-21)


### Features

* quick-filter to current item's namespace from selector ([#455](https://github.com/janosmiko/lfk/issues/455)) ([#456](https://github.com/janosmiko/lfk/issues/456)) ([35ac6c3](https://github.com/janosmiko/lfk/commit/35ac6c37914fd3391ef36a1df8fa4fc4a9c60eca))

## [0.14.12](https://github.com/janosmiko/lfk/compare/v0.14.11...v0.14.12) (2026-06-19)


### Features

* Log Top — live log aggregation view (top-style request analytics) ([#453](https://github.com/janosmiko/lfk/issues/453)) ([fd79482](https://github.com/janosmiko/lfk/commit/fd794829a97197f0e2333545c9aa4c9263e78286))

## [0.14.11](https://github.com/janosmiko/lfk/compare/v0.14.10...v0.14.11) (2026-06-17)


### Bug Fixes

* **templates:** make workloads + Service/Ingress compose out of the box ([#451](https://github.com/janosmiko/lfk/issues/451)) ([44bd98b](https://github.com/janosmiko/lfk/commit/44bd98b8bc63fe7bcc0bddd19606025f08ad1c29))

## [0.14.10](https://github.com/janosmiko/lfk/compare/v0.14.9...v0.14.10) (2026-06-17)


### Features

* allow Ctrl+O (open in browser) for port forwards ([#445](https://github.com/janosmiko/lfk/issues/445)) ([fc38e92](https://github.com/janosmiko/lfk/commit/fc38e9241d187f7d9eb54be6d26569a86e392775))
* Port Forward & Open action for Services (Ctrl+O) ([#448](https://github.com/janosmiko/lfk/issues/448)) ([691b612](https://github.com/janosmiko/lfk/commit/691b612c24224c92094b48bcf6a6cab79819ca99))


### Bug Fixes

* **logs:** reconnect specific-container stream while it is waiting to start ([#450](https://github.com/janosmiko/lfk/issues/450)) ([7e61834](https://github.com/janosmiko/lfk/commit/7e61834364fc50c9522fa59b14e61e47a799acbf))
* scope to default namespace when toggling all-namespaces off with none set ([#449](https://github.com/janosmiko/lfk/issues/449)) ([ea9d9b2](https://github.com/janosmiko/lfk/commit/ea9d9b26217eb23b18b5a44cf788739cf0febcb2))
* show PriorityClass Value and Preemption Policy columns ([#446](https://github.com/janosmiko/lfk/issues/446)) ([c8ef02c](https://github.com/janosmiko/lfk/commit/c8ef02c001f0209afa0e00b619a3c1f315e7dfe2))

## [0.14.9](https://github.com/janosmiko/lfk/compare/v0.14.8...v0.14.9) (2026-06-17)


### Features

* force-delete and evict replicas for Longhorn nodes ([#443](https://github.com/janosmiko/lfk/issues/443)) ([fbb4763](https://github.com/janosmiko/lfk/commit/fbb47633ec56ba7dd6f8aab8ada9213582eb5c41))
* **logs:** filtering, severity filter, and current-match highlight ([#442](https://github.com/janosmiko/lfk/issues/442)) ([9cc3c72](https://github.com/janosmiko/lfk/commit/9cc3c72dd2dc72f77ea242a778d655ea5c3c9d9d))

## [0.14.8](https://github.com/janosmiko/lfk/compare/v0.14.7...v0.14.8) (2026-06-15)


### Bug Fixes

* **namespace:** restore selection when leaving all-namespaces mode ([#433](https://github.com/janosmiko/lfk/issues/433)) ([#434](https://github.com/janosmiko/lfk/issues/434)) ([512ee4a](https://github.com/janosmiko/lfk/commit/512ee4a22ec8552161e9b2d4e770edf702d963b5))

## [0.14.7](https://github.com/janosmiko/lfk/compare/v0.14.6...v0.14.7) (2026-06-14)


### Features

* **security:** label-match ignore patterns for findings ([#428](https://github.com/janosmiko/lfk/issues/428)) ([#432](https://github.com/janosmiko/lfk/issues/432)) ([4cbc43a](https://github.com/janosmiko/lfk/commit/4cbc43a4019f42521d54b5aee1980269ac25827e))


### Bug Fixes

* **scheduler:** wire unscheduled K8s reads through the task scheduler ([#429](https://github.com/janosmiko/lfk/issues/429)) ([9574ad5](https://github.com/janosmiko/lfk/commit/9574ad53871634ce13aa5dc14cb4eb0f05c6db4c))

## [0.14.6](https://github.com/janosmiko/lfk/compare/v0.14.5...v0.14.6) (2026-06-12)


### Bug Fixes

* **logs:** give each container a distinct prefix color in the log viewer ([#425](https://github.com/janosmiko/lfk/issues/425)) ([be5b259](https://github.com/janosmiko/lfk/commit/be5b259b71b7dc5553f6941a58ac5c562a03e147))

## [0.14.5](https://github.com/janosmiko/lfk/compare/v0.14.4...v0.14.5) (2026-06-12)


### Bug Fixes

* action menu hotkey order, overlay badge background, tree-view descriptions ([#423](https://github.com/janosmiko/lfk/issues/423)) ([6ae13b5](https://github.com/janosmiko/lfk/commit/6ae13b5a24d4575297f07145c56bad086dc49650))

## [0.14.4](https://github.com/janosmiko/lfk/compare/v0.14.3...v0.14.4) (2026-06-11)


### Features

* **explorer:** ASCII-art tree view for Object and API Explorers ([#420](https://github.com/janosmiko/lfk/issues/420)) ([c90bd12](https://github.com/janosmiko/lfk/commit/c90bd12c6d72b721f49152e76f99af5f1e062700))
* **logs:** live-log preview for container rows ([#419](https://github.com/janosmiko/lfk/issues/419)) ([729ed9d](https://github.com/janosmiko/lfk/commit/729ed9d17b387d046f6061daf4fa5e98bfe54e62))
* **netpol:** CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy support ([#414](https://github.com/janosmiko/lfk/issues/414)) ([e8b3d06](https://github.com/janosmiko/lfk/commit/e8b3d06549ee14171bac7da34368b0be81dc4fa1)), closes [#409](https://github.com/janosmiko/lfk/issues/409)
* **netpol:** mouse wheel scroll and / search in the policy visualizer ([#421](https://github.com/janosmiko/lfk/issues/421)) ([6a08d6e](https://github.com/janosmiko/lfk/commit/6a08d6efac767354c61be849f978caa9d576613c))
* **netpol:** show network policies affecting a pod or service ([#412](https://github.com/janosmiko/lfk/issues/412)) ([914a635](https://github.com/janosmiko/lfk/commit/914a635a8dc34e4aa2207c888ddddf68e2643352))
* **security:** 37 new built-in checks, RBAC source, and finding-jump fixes ([#413](https://github.com/janosmiko/lfk/issues/413)) ([8ed8e5d](https://github.com/janosmiko/lfk/commit/8ed8e5d0254254bd20b1e9629c3e134f980150dc))
* **security:** Security Findings action opens a per-resource findings list ([#418](https://github.com/janosmiko/lfk/issues/418)) ([55f4ca8](https://github.com/janosmiko/lfk/commit/55f4ca81c7c2d8ad7d991c9b113b91deb493b12c))
* **tips:** promote lesser-known features in the startup tip bar ([#416](https://github.com/janosmiko/lfk/issues/416)) ([70c8e7d](https://github.com/janosmiko/lfk/commit/70c8e7dbee2b41f64285d4fb517925660a6fc68c))


### Bug Fixes

* **preview:** follow the list's sorting and column config in list previews ([#410](https://github.com/janosmiko/lfk/issues/410)) ([a7b7dc7](https://github.com/janosmiko/lfk/commit/a7b7dc7f97e6b61f4c39b38529af70bd47e542b7))
* **preview:** stop column flicker by carrying enriched columns into preview refetches ([#422](https://github.com/janosmiko/lfk/issues/422)) ([cd787b2](https://github.com/janosmiko/lfk/commit/cd787b2f41bc343ea24c4a51271859a7e9495487))

## [0.14.3](https://github.com/janosmiko/lfk/compare/v0.14.2...v0.14.3) (2026-06-10)


### Features

* **preview:** live-log preview in the explorer right pane ([#405](https://github.com/janosmiko/lfk/issues/405)) ([9bc6994](https://github.com/janosmiko/lfk/commit/9bc6994de8243340ed286ef4199f2bdfe01550db))
* **security:** advisor source with reliability recommendations ([#403](https://github.com/janosmiko/lfk/issues/403)) ([51320eb](https://github.com/janosmiko/lfk/commit/51320eb87e432a5079e25946c66cbdde8ef84b4c))
* **security:** extended heuristic checks + tunable secret_env ([#402](https://github.com/janosmiko/lfk/issues/402)) ([34763bb](https://github.com/janosmiko/lfk/commit/34763bbc1c7e6d22f77d6385c7494a8aa250bee3))


### Bug Fixes

* **exec:** use cmd.exe/PowerShell when exec-ing into Windows pods ([#407](https://github.com/janosmiko/lfk/issues/407)) ([b078fdf](https://github.com/janosmiko/lfk/commit/b078fdfa1c73fa2693f5eb37622487c2e8b61c13)), closes [#406](https://github.com/janosmiko/lfk/issues/406)
* pin the affected-resources table while finding-group details scroll ([#404](https://github.com/janosmiko/lfk/issues/404)) ([473f61f](https://github.com/janosmiko/lfk/commit/473f61fd4b22ebc4e21c3cc2fd47ac67f8994d08))
* stop preview-pane wheel from scrolling the pane to its left ([#398](https://github.com/janosmiko/lfk/issues/398)) ([#399](https://github.com/janosmiko/lfk/issues/399)) ([24318b0](https://github.com/janosmiko/lfk/commit/24318b00ecfb8e58ec8d861988b629bcfd501659))

## [0.14.2](https://github.com/janosmiko/lfk/compare/v0.14.1...v0.14.2) (2026-06-09)


### Bug Fixes

* live-refresh the Object Explorer ([#391](https://github.com/janosmiko/lfk/issues/391)) ([#394](https://github.com/janosmiko/lfk/issues/394)) ([7e64bca](https://github.com/janosmiko/lfk/commit/7e64bca3bb204e60500197fd5fd26ae5cd881c67))
* **logs:** bound live log buffer to stop unbounded memory growth ([#387](https://github.com/janosmiko/lfk/issues/387)) ([#392](https://github.com/janosmiko/lfk/issues/392)) ([a96ac1c](https://github.com/janosmiko/lfk/commit/a96ac1c4214ed601cd54da8f37ad006e674d6f31))

## [0.14.1](https://github.com/janosmiko/lfk/compare/v0.14.0...v0.14.1) (2026-06-08)


### Bug Fixes

* **security:** eliminate findings-cache memory spike ([#387](https://github.com/janosmiko/lfk/issues/387)) ([#390](https://github.com/janosmiko/lfk/issues/390)) ([548fa42](https://github.com/janosmiko/lfk/commit/548fa424efa5b1ded61da4f981d232e97ae617b5))
* wrap long lines in diff viewer instead of truncating ([#386](https://github.com/janosmiko/lfk/issues/386)) ([#388](https://github.com/janosmiko/lfk/issues/388)) ([b2ce7ab](https://github.com/janosmiko/lfk/commit/b2ce7ab8425c0e987e1223052703b6d24d6fb679))

## [0.14.0](https://github.com/janosmiko/lfk/compare/v0.13.9...v0.14.0) (2026-06-08)


### ⚠ BREAKING CHANGES

* the flat keys log_tail_lines, log_tail_lines_short, log_render_ansi, colorscheme, icons, no_color, transparent_background, min_contrast_ratio and dim_overlay are deprecated in favour of their grouped equivalents (log_viewer.*, appearance.*). They continue to work as aliases for now, but the grouped form is canonical and the flat keys may be removed in a future release. Migrate config.yaml to the grouped shape; when both a flat key and its group equivalent are set, the group wins.

### Features

* group log, viewer, session and appearance settings into config sections ([#378](https://github.com/janosmiko/lfk/issues/378)) ([e5ba655](https://github.com/janosmiko/lfk/commit/e5ba655f88d3140d4bd75cfca1a075d811d50fd0))
* unify viewer keybindings — configurable line-wrap, display toggles, search/help/match, and Shift+F fullscreen ([#380](https://github.com/janosmiko/lfk/issues/380)) ([90b8db6](https://github.com/janosmiko/lfk/commit/90b8db62db2990c58f9027c55d2155e9a5e2f650))


### Bug Fixes

* route mouse wheel to the pane under the pointer in Object Explorer and log viewer ([#382](https://github.com/janosmiko/lfk/issues/382)) ([0a37575](https://github.com/janosmiko/lfk/commit/0a37575cc8320b5f8a77af0b8ec81bfeff2a9ee6))

## [0.13.9](https://github.com/janosmiko/lfk/compare/v0.13.8...v0.13.9) (2026-06-07)


### Features

* add JSON Schema for config.yaml with editor autocompletion ([#376](https://github.com/janosmiko/lfk/issues/376)) ([c58ea57](https://github.com/janosmiko/lfk/commit/c58ea579d673cc6532560379c6e3b43be5fcc506))
* add show_rare_types config to show all resource types from startup ([#321](https://github.com/janosmiko/lfk/issues/321)) ([#374](https://github.com/janosmiko/lfk/issues/374)) ([8dd78c5](https://github.com/janosmiko/lfk/commit/8dd78c574955a42520ed1cd36b5ab37b314010d3))

## [0.13.8](https://github.com/janosmiko/lfk/compare/v0.13.7...v0.13.8) (2026-06-07)


### Features

* alias shift+down/shift+up to ctrl+d/ctrl+u half-page scroll (closes [#369](https://github.com/janosmiko/lfk/issues/369)) ([#371](https://github.com/janosmiko/lfk/issues/371)) ([15eaf36](https://github.com/janosmiko/lfk/commit/15eaf369cbdc2d1f7cb0a356dc5ac5fd650c7d3b))
* YAML viewer O-jump to Object Explorer + attribute path in title ([#372](https://github.com/janosmiko/lfk/issues/372)) ([4b670a3](https://github.com/janosmiko/lfk/commit/4b670a3ef605fa3fe532fea7956eb2396459f917))

## [0.13.7](https://github.com/janosmiko/lfk/compare/v0.13.6...v0.13.7) (2026-06-05)


### Features

* Object Explorer — browse a resource's live object (closes [#361](https://github.com/janosmiko/lfk/issues/361)) ([#366](https://github.com/janosmiko/lfk/issues/366)) ([f626a77](https://github.com/janosmiko/lfk/commit/f626a7793232ada0933a5400626f5b152b91bd52))

## [0.13.6](https://github.com/janosmiko/lfk/compare/v0.13.5...v0.13.6) (2026-06-05)


### Features

* **preview:** generic status summary for any kind with phase/conditions ([#352](https://github.com/janosmiko/lfk/issues/352)) ([#364](https://github.com/janosmiko/lfk/issues/364)) ([65a4126](https://github.com/janosmiko/lfk/commit/65a412679f6207a238b94d326a1ec5c7fd35d500))


### Bug Fixes

* **preview:** let the right pane scroll long lists to the bottom ([#365](https://github.com/janosmiko/lfk/issues/365)) ([06a9c27](https://github.com/janosmiko/lfk/commit/06a9c27fcd409cd50932175c66e2a618a901f629))
* **ui:** compress Name so configured columns survive the three-pane list ([#354](https://github.com/janosmiko/lfk/issues/354)) ([#362](https://github.com/janosmiko/lfk/issues/362)) ([98ac369](https://github.com/janosmiko/lfk/commit/98ac369695bf5ba238758628c5e94c3c422c995c))

## [0.13.5](https://github.com/janosmiko/lfk/compare/v0.13.4...v0.13.5) (2026-06-03)


### Features

* persist resource-list sort order and column layout across restarts ([#359](https://github.com/janosmiko/lfk/issues/359)) ([337eaba](https://github.com/janosmiko/lfk/commit/337eabacd00ee493ccf594b43806645e76575406))
* **preview:** list status summary band in resource-type preview ([#360](https://github.com/janosmiko/lfk/issues/360)) ([e5bc62c](https://github.com/janosmiko/lfk/commit/e5bc62c2670c3800b46c0325516efc17f4eb87e5))
* **ui:** make Name a configurable resource-list column ([#356](https://github.com/janosmiko/lfk/issues/356)) ([dcc5bd2](https://github.com/janosmiko/lfk/commit/dcc5bd2e4906c38a37f660c6589f3891bd69f8ac))


### Bug Fixes

* **yaml:** keep long scalars on one line in resource view ([#355](https://github.com/janosmiko/lfk/issues/355)) ([#357](https://github.com/janosmiko/lfk/issues/357)) ([419f6f7](https://github.com/janosmiko/lfk/commit/419f6f7dfde068536597e76a3ae823c6f9474611))

## [0.13.4](https://github.com/janosmiko/lfk/compare/v0.13.3...v0.13.4) (2026-06-02)


### Features

* **dashboard:** show pod capacity headroom in cluster pod bar ([#345](https://github.com/janosmiko/lfk/issues/345)) ([a60341f](https://github.com/janosmiko/lfk/commit/a60341fb0156ead1290c6840d8e80c6bff0f9f20)), closes [#342](https://github.com/janosmiko/lfk/issues/342)
* **preview:** show full condition detail with severity coloring ([#340](https://github.com/janosmiko/lfk/issues/340)) ([#351](https://github.com/janosmiko/lfk/issues/351)) ([fe50e90](https://github.com/janosmiko/lfk/commit/fe50e905a032765b06acff3ecbcc434ad1f81dfe))
* **ui:** sort-column highlight and width-aware columns ([#350](https://github.com/janosmiko/lfk/issues/350)) ([0ba2c5b](https://github.com/janosmiko/lfk/commit/0ba2c5b864dce9fd9d82a17c380e141919007471))


### Bug Fixes

* keep sort cycling stable when sort column is hidden ([#339](https://github.com/janosmiko/lfk/issues/339)) ([#346](https://github.com/janosmiko/lfk/issues/346)) ([8e256c9](https://github.com/janosmiko/lfk/commit/8e256c9eda6e2377d2d82bb91b3bd5c46574fa3b))
* update nix vendorHash for go modules ([#343](https://github.com/janosmiko/lfk/issues/343)) ([73c264e](https://github.com/janosmiko/lfk/commit/73c264e0fa08a85dd7a9e9bf3b5d45914074757b)), closes [#341](https://github.com/janosmiko/lfk/issues/341)

## [0.13.3](https://github.com/janosmiko/lfk/compare/v0.13.2...v0.13.3) (2026-06-01)


### Features

* **app:** wrap application log lines and add events-style cursor navigation ([#325](https://github.com/janosmiko/lfk/issues/325)) ([48ef73f](https://github.com/janosmiko/lfk/commit/48ef73f38c644b039a5cda84a0b8ed33ca50b6c0))
* **help:** word-wrap long keybinding descriptions ([#319](https://github.com/janosmiko/lfk/issues/319) a) ([#329](https://github.com/janosmiko/lfk/issues/329)) ([13b3e9f](https://github.com/janosmiko/lfk/commit/13b3e9fbd5277c40eafd11f04f5a07a56b2b2e5b))
* hide individual resource types per cluster ([#321](https://github.com/janosmiko/lfk/issues/321)) ([#338](https://github.com/janosmiko/lfk/issues/338)) ([8535628](https://github.com/janosmiko/lfk/commit/85356280292126a25a6758ea9066cfd33e7ef74d))
* metrics loading placeholder and segmented resource-usage bars ([#324](https://github.com/janosmiko/lfk/issues/324)) ([7ba0ed1](https://github.com/janosmiko/lfk/commit/7ba0ed1edd80b4376a77a4453d93dbcc7d4068c4))
* **mouse:** add a runtime mouse-capture toggle ([#331](https://github.com/janosmiko/lfk/issues/331)) ([2f78dd1](https://github.com/janosmiko/lfk/commit/2f78dd13640dea1015c35ef1196aefdf9403679f))
* **mouse:** scroll the pane under the pointer ([#330](https://github.com/janosmiko/lfk/issues/330)) ([d58ee21](https://github.com/janosmiko/lfk/commit/d58ee21eabbd71ed2ca6016518434c9051501cee))
* **security:** finding-ignore visibility — namespace + config-glob ignores, badge/action consistency, cached lists ([#332](https://github.com/janosmiko/lfk/issues/332)) ([6d3411e](https://github.com/janosmiko/lfk/commit/6d3411ed5be60a0d085bac9e4cd40bd7d9d73499))
* **security:** security findings dashboard ([#183](https://github.com/janosmiko/lfk/issues/183)) ([2e3c0d9](https://github.com/janosmiko/lfk/commit/2e3c0d968f86977436bc08368b06e9959cbca268))


### Bug Fixes

* **app:** make filter/search inert in dashboard fullscreen ([#323](https://github.com/janosmiko/lfk/issues/323)) ([6db089c](https://github.com/janosmiko/lfk/commit/6db089c3a5b62e8481b109fca0a982a44fb9d085))
* attribute cluster credential failures + Prometheus pod metrics + log viewer cursor ([#318](https://github.com/janosmiko/lfk/issues/318)) ([ce26c05](https://github.com/janosmiko/lfk/commit/ce26c05f2f568a4771186e38fda00bf595db6fc6))
* GUI freeze while loading background jobs ([#328](https://github.com/janosmiko/lfk/issues/328)) ([7237559](https://github.com/janosmiko/lfk/commit/723755905fab66e6ca6f6cb75a466b74868ca333))
* **scheduler:** reclaim superseded background work so the focused view wins ([#317](https://github.com/janosmiko/lfk/issues/317)) ([0e18880](https://github.com/janosmiko/lfk/commit/0e188808a1a840c729fecfd53d054c3bb4196cb2))
* show loader on startup instead of flashing empty states ([#333](https://github.com/janosmiko/lfk/issues/333)) ([70190d3](https://github.com/janosmiko/lfk/commit/70190d384706f7f1707ba791973e8b24ed259af7))
* **ui:** refresh all rows immediately on SEC badge toggle ([#326](https://github.com/janosmiko/lfk/issues/326)) ([7cece15](https://github.com/janosmiko/lfk/commit/7cece1561dcb4ff83dcc6ace50cb42ba71f88c36))
* **ui:** sort CPU/MEM columns numerically, n/a last ([#327](https://github.com/janosmiko/lfk/issues/327)) ([8279452](https://github.com/janosmiko/lfk/commit/82794523614f3eb87ee2a9fd3ab5f268124aa301))


### Performance Improvements

* fix background-work starvation, cache clients, configurable rate limits ([#322](https://github.com/janosmiko/lfk/issues/322)) ([5249fe9](https://github.com/janosmiko/lfk/commit/5249fe9660e39bed65a8b7e640784c3be39caded))

## [0.13.2](https://github.com/janosmiko/lfk/compare/v0.13.1...v0.13.2) (2026-05-31)


### Features

* graceful shutdown notice with 10s force-quit timeout ([#314](https://github.com/janosmiko/lfk/issues/314)) ([68e03f6](https://github.com/janosmiko/lfk/commit/68e03f62ce7f8a279112a4f271efbf52228f53f9))

## [0.13.1](https://github.com/janosmiko/lfk/compare/v0.13.0...v0.13.1) (2026-05-31)


### Features

* derive CRD display names from Kind to preserve camel case ([#306](https://github.com/janosmiko/lfk/issues/306)) ([c334c85](https://github.com/janosmiko/lfk/commit/c334c85b571e9d867aaf223bfbc8f27b0ff30cba)), closes [#301](https://github.com/janosmiko/lfk/issues/301)
* per-kind sort memory and per-context column memory ([#310](https://github.com/janosmiko/lfk/issues/310)) ([4e80556](https://github.com/janosmiko/lfk/commit/4e80556ba5e10368ac63804ef5798c80530259bf))


### Bug Fixes

* honor CRD additionalPrinterColumns priority and keep them visible ([#309](https://github.com/janosmiko/lfk/issues/309)) ([bed3a7a](https://github.com/janosmiko/lfk/commit/bed3a7af0d241c34bae675af80d0c1c93544f199))
* link manually-triggered CronJob jobs to their CronJob via ownerRef ([#308](https://github.com/janosmiko/lfk/issues/308)) ([39acb48](https://github.com/janosmiko/lfk/commit/39acb485a3391fc48491481fc09e2a348db52594)), closes [#304](https://github.com/janosmiko/lfk/issues/304)
* remember resource list filter across subview navigation ([#303](https://github.com/janosmiko/lfk/issues/303)) ([#311](https://github.com/janosmiko/lfk/issues/311)) ([4dca616](https://github.com/janosmiko/lfk/commit/4dca616fb12776e7e21ba3f3641e731e82e8e729))

## [0.13.0](https://github.com/janosmiko/lfk/compare/v0.12.9...v0.13.0) (2026-05-30)


### Features

* pin individual resource types into a top "Pinned" section ([#300](https://github.com/janosmiko/lfk/issues/300)) ([3953bd6](https://github.com/janosmiko/lfk/commit/3953bd63416313413705a420b19e8daf32a9b3d2))


### Bug Fixes

* apply theme switch to cached previews immediately ([#299](https://github.com/janosmiko/lfk/issues/299)) ([7d32aaa](https://github.com/janosmiko/lfk/commit/7d32aaa028d3fcc8903d962edb251cdce050076b))
* dashboard events preview background tear under non-black themes ([#293](https://github.com/janosmiko/lfk/issues/293)) ([#296](https://github.com/janosmiko/lfk/issues/296)) ([2d1672d](https://github.com/janosmiko/lfk/commit/2d1672dc7272759cf72904a93982f4c4d4fc644a))

## [0.12.9](https://github.com/janosmiko/lfk/compare/v0.12.8...v0.12.9) (2026-05-29)


### Bug Fixes

* cluster dashboard rendering, scrolling, and responsive usage bars ([#293](https://github.com/janosmiko/lfk/issues/293)) ([#294](https://github.com/janosmiko/lfk/issues/294)) ([9b3fb00](https://github.com/janosmiko/lfk/commit/9b3fb001d4a32f2a7e51780f5dda4e1a1cffae43))

## [0.12.8](https://github.com/janosmiko/lfk/compare/v0.12.7...v0.12.8) (2026-05-28)


### Features

* refresh namespaces in the namespace selector with R ([#292](https://github.com/janosmiko/lfk/issues/292)) ([bc6ace6](https://github.com/janosmiko/lfk/commit/bc6ace6aaf876eda1a5c48260e909a2c421a2db6))


### Bug Fixes

* clear hint-bar message immediately on explorer navigation ([#291](https://github.com/janosmiko/lfk/issues/291)) ([3b5bb3e](https://github.com/janosmiko/lfk/commit/3b5bb3e628cb1aaa55b7de899250372174d707ae))
* gate explorer hint bar by nav level at context picker ([#288](https://github.com/janosmiko/lfk/issues/288)) ([199de4b](https://github.com/janosmiko/lfk/commit/199de4b9730e1a00e349d14b84daec7bb662c874))
* persist in-context read-only toggle to per-context override ([#290](https://github.com/janosmiko/lfk/issues/290)) ([35cbab5](https://github.com/janosmiko/lfk/commit/35cbab5dfdc868303c7300e8a6359a91dc60bb20))

## [0.12.7](https://github.com/janosmiko/lfk/compare/v0.12.6...v0.12.7) (2026-05-28)


### Features

* add negative namespace selection ([#287](https://github.com/janosmiko/lfk/issues/287)) ([653d1cd](https://github.com/janosmiko/lfk/commit/653d1cd9d83708ffa727580af4cd03fc5343c302))


### Bug Fixes

* **nix:** update vendorHash after k8s dependency bump ([#285](https://github.com/janosmiko/lfk/issues/285)) ([9020c87](https://github.com/janosmiko/lfk/commit/9020c87fe35b4d8b60cfae0c23ab8ed9346bef5d)), closes [#284](https://github.com/janosmiko/lfk/issues/284)

## [0.12.6](https://github.com/janosmiko/lfk/compare/v0.12.5...v0.12.6) (2026-05-25)


### Bug Fixes

* **views:** apply GVR-keyed view columns and hide unlisted builtins ([#277](https://github.com/janosmiko/lfk/issues/277)) ([a2df863](https://github.com/janosmiko/lfk/commit/a2df863902a2f58f6c9dd4ed896a210d43da2051)), closes [#262](https://github.com/janosmiko/lfk/issues/262)

## [0.12.5](https://github.com/janosmiko/lfk/compare/v0.12.4...v0.12.5) (2026-05-25)


### Features

* **columns + views:** REV, kubectl-parity audit, k9s-style views config ([#271](https://github.com/janosmiko/lfk/issues/271)) ([837961c](https://github.com/janosmiko/lfk/commit/837961ce44c1de45eca45e17be0a9f916e86dbe2))

## [0.12.4](https://github.com/janosmiko/lfk/compare/v0.12.3...v0.12.4) (2026-05-24)


### Bug Fixes

* **sort:** numeric ordering for percent columns (CPU%, MEM%, */R, */L) ([#273](https://github.com/janosmiko/lfk/issues/273)) ([f50537a](https://github.com/janosmiko/lfk/commit/f50537a8cc91fee15fabad596f671182922a7b58)), closes [#272](https://github.com/janosmiko/lfk/issues/272)

## [0.12.3](https://github.com/janosmiko/lfk/compare/v0.12.2...v0.12.3) (2026-05-23)


### Features

* **actions:** add "Go to Node" to the Pod action menu ([#264](https://github.com/janosmiko/lfk/issues/264)) ([#269](https://github.com/janosmiko/lfk/issues/269)) ([c9190d1](https://github.com/janosmiko/lfk/commit/c9190d1dbc87e5e4f0bb956e058f5be02cdd1b9b))
* **events:** make the events overlay readable by default ([#263](https://github.com/janosmiko/lfk/issues/263)) ([#270](https://github.com/janosmiko/lfk/issues/270)) ([8bce4ea](https://github.com/janosmiko/lfk/commit/8bce4ea2afc704d655e48f9fa7908feaa124bc58))
* **logger:** surface silent failures with dedup to in-app log ([#268](https://github.com/janosmiko/lfk/issues/268)) ([ccfece0](https://github.com/janosmiko/lfk/commit/ccfece0682b69b165c21edae994320d8440bba25))


### Bug Fixes

* **metrics:** fall back to metrics-api when prometheus route fails ([#266](https://github.com/janosmiko/lfk/issues/266)) ([3a79c37](https://github.com/janosmiko/lfk/commit/3a79c377b9abb45019e3029066fe2cee840c4518))
* normalize clipboard line endings to CRLF on Windows ([#261](https://github.com/janosmiko/lfk/issues/261)) ([#265](https://github.com/janosmiko/lfk/issues/265)) ([2b884aa](https://github.com/janosmiko/lfk/commit/2b884aa610a49b90bc3b08ed9ce29919e5d2e2d1))

## [0.12.2](https://github.com/janosmiko/lfk/compare/v0.12.1...v0.12.2) (2026-05-21)


### Bug Fixes

* **bulk:** clear stale bulk-action snapshot on dispatch and cancel ([#257](https://github.com/janosmiko/lfk/issues/257)) ([16a4c60](https://github.com/janosmiko/lfk/commit/16a4c606b632785b84f383251d88b31d12fd7a47))
* stop node metrics column-order flicker (and prevent the whole class) ([#259](https://github.com/janosmiko/lfk/issues/259)) ([7f5c695](https://github.com/janosmiko/lfk/commit/7f5c695020bee63ed35db09ef20992a31cece50a))

## [0.12.1](https://github.com/janosmiko/lfk/compare/v0.12.0...v0.12.1) (2026-05-19)


### Features

* add jump-back navigation history ([#249](https://github.com/janosmiko/lfk/issues/249)) ([#256](https://github.com/janosmiko/lfk/issues/256)) ([c7eae4b](https://github.com/janosmiko/lfk/commit/c7eae4ba6828bcb1abca2a50b63fac6aae449835))


### Bug Fixes

* **port-forward:** reuse local port on restart, fix setup-overlay UX ([#253](https://github.com/janosmiko/lfk/issues/253)) ([d065a5e](https://github.com/janosmiko/lfk/commit/d065a5e15204f9edfb55314a8186420513eb3d59))
* **sort:** sort numeric/structured columns numerically ([#255](https://github.com/janosmiko/lfk/issues/255)) ([1a13ad0](https://github.com/janosmiko/lfk/commit/1a13ad03893d239819238cc4266765a85ef9d8b6)), closes [#250](https://github.com/janosmiko/lfk/issues/250)

## [0.12.0](https://github.com/janosmiko/lfk/compare/v0.11.8...v0.12.0) (2026-05-18)


### Features

* add configurable data directories (LFK_*_DIR overrides) ([#246](https://github.com/janosmiko/lfk/issues/246)) ([4de5317](https://github.com/janosmiko/lfk/commit/4de531735ea2d480165f0d19c0760061ce5ec798))
* add multi-cluster union view with --union-context and --union-set ([#172](https://github.com/janosmiko/lfk/issues/172)) ([ba0f405](https://github.com/janosmiko/lfk/commit/ba0f4059db6cd6588a4e4d8c9c4ba77fadd543db))

## [0.11.8](https://github.com/janosmiko/lfk/compare/v0.11.7...v0.11.8) (2026-05-15)


### Features

* **config:** make kubeconfig discovery directory configurable ([#243](https://github.com/janosmiko/lfk/issues/243)) ([71fddf5](https://github.com/janosmiko/lfk/commit/71fddf5c43e18019c7affd934a72372403c36210))

## [0.11.7](https://github.com/janosmiko/lfk/compare/v0.11.6...v0.11.7) (2026-05-14)


### Bug Fixes

* **logs:** keep cursor at top when older history loads after gg ([#241](https://github.com/janosmiko/lfk/issues/241)) ([add9187](https://github.com/janosmiko/lfk/commit/add918752cdcd067bc4151bbcee28bd4b0bff1c9))

## [0.11.6](https://github.com/janosmiko/lfk/compare/v0.11.5...v0.11.6) (2026-05-14)


### Features

* **copy:** open copy-as picker on Y with YAML / JSON / Table options ([#237](https://github.com/janosmiko/lfk/issues/237)) ([9f0851d](https://github.com/janosmiko/lfk/commit/9f0851d020c96ae40bf50b63b48366ca923fc1b8))
* **ui:** migrate HelmHistory / HelmRollback / DeploymentRollback to OverlayList ([#234](https://github.com/janosmiko/lfk/issues/234)) ([1fbe874](https://github.com/janosmiko/lfk/commit/1fbe87495036b72b7c9ec1646810911086f81450))
* **ui:** unified overlay components — OverlayList, OverlayConfirm, OverlayInput ([#231](https://github.com/janosmiko/lfk/issues/231)) ([5960090](https://github.com/janosmiko/lfk/commit/59600903183f6a7891282e3c23e335567c6b4362))


### Bug Fixes

* **commandbar:** -A flag, pty inline output, exact-match autocomplete ([#235](https://github.com/janosmiko/lfk/issues/235)) ([040a690](https://github.com/janosmiko/lfk/commit/040a6901b17616cb7df3adb0ab98652111eef912))
* **copy:** JSON shortcut chip + partial-success on bulk YAML/JSON copy ([#239](https://github.com/janosmiko/lfk/issues/239)) ([74ec4df](https://github.com/janosmiko/lfk/commit/74ec4df9e173f6a30764015d89ffac765c3f4f75))
* details pane shows children + theme color tracking ([#238](https://github.com/janosmiko/lfk/issues/238)) ([61e0ca3](https://github.com/janosmiko/lfk/commit/61e0ca3f6a42213352d715180cf28cd071ca5e8c))
* **kv-editor:** dedicated selection column for the Secret/ConfigMap/Label editors ([#240](https://github.com/janosmiko/lfk/issues/240)) ([2c00356](https://github.com/janosmiko/lfk/commit/2c00356b486f11e5d50626dbcdf819cb4f84dca2))

## [0.11.5](https://github.com/janosmiko/lfk/compare/v0.11.4...v0.11.5) (2026-05-13)


### Features

* **filters:** add Not Running / Not Bound presets and config invert flag ([#230](https://github.com/janosmiko/lfk/issues/230)) ([3af1652](https://github.com/janosmiko/lfk/commit/3af16528ec28973e4e0893fb47754aa48a050396))


### Bug Fixes

* **nix:** build with Go 1.26.3 by overriding pkgs.go_1_26 ([#228](https://github.com/janosmiko/lfk/issues/228)) ([f4d0ad6](https://github.com/janosmiko/lfk/commit/f4d0ad6c3b8b527af26171fca757886f4ff72d21))

## [0.11.4](https://github.com/janosmiko/lfk/compare/v0.11.3...v0.11.4) (2026-05-12)


### Features

* **karpenter:** first-class actions for NodePool / NodeClaim / EC2NodeClass ([#223](https://github.com/janosmiko/lfk/issues/223)) ([5f37b70](https://github.com/janosmiko/lfk/commit/5f37b70e21a4edf09aee9416332784e41ed3c15a))
* **knative:** first-class Knative Serving (Activate) + Eventing icons ([#224](https://github.com/janosmiko/lfk/issues/224)) ([e89be5b](https://github.com/janosmiko/lfk/commit/e89be5baa1405e366334a52bb999e494aad63e74))


### Bug Fixes

* **app:** respect KUBE_EDITOR and parse editor flags ([#226](https://github.com/janosmiko/lfk/issues/226)) ([d944265](https://github.com/janosmiko/lfk/commit/d9442656a5be06685742a63596bfd867ba879fb4))
* **scheduler:** stop title-bar spinner during 10s linger window after work completes ([#220](https://github.com/janosmiko/lfk/issues/220)) ([f441c78](https://github.com/janosmiko/lfk/commit/f441c7844e3bcd0450a3c1f11445633ed7879c79))

## [0.11.3](https://github.com/janosmiko/lfk/compare/v0.11.2...v0.11.3) (2026-05-12)


### Bug Fixes

* **app:** refresh right-pane preview at LevelResourceTypes on tab switch and watch tick ([#216](https://github.com/janosmiko/lfk/issues/216)) ([cc4c90f](https://github.com/janosmiko/lfk/commit/cc4c90f68d5e4d73c6e8915bf340fd61335c73b8))
* **ui:** drop blank line between RESOURCE USAGE header and bars ([#217](https://github.com/janosmiko/lfk/issues/217)) ([40be2bc](https://github.com/janosmiko/lfk/commit/40be2bc9148c41d961e1fadd0c786dfb8854a150))


### Reverts

* **app:** remove spinner tick-chain gate from [#206](https://github.com/janosmiko/lfk/issues/206) fix ([#215](https://github.com/janosmiko/lfk/issues/215)) ([2ab72d2](https://github.com/janosmiko/lfk/commit/2ab72d2bd5fcdb7c576d71e6929b910b6abb1eca))

## [0.11.2](https://github.com/janosmiko/lfk/compare/v0.11.1...v0.11.2) (2026-05-11)


### Bug Fixes

* **app,scheduler:** drop idle CPU from ~145% to ~0% (closes [#206](https://github.com/janosmiko/lfk/issues/206)) ([#211](https://github.com/janosmiko/lfk/issues/211)) ([bcdd3d0](https://github.com/janosmiko/lfk/commit/bcdd3d0ce10cb924a4e58703cc64367ddb7c5688))

## [0.11.1](https://github.com/janosmiko/lfk/compare/v0.11.0...v0.11.1) (2026-05-11)


### Features

* **clipboard:** support Windows and Wayland via atotto/clipboard ([#195](https://github.com/janosmiko/lfk/issues/195)) ([c1871de](https://github.com/janosmiko/lfk/commit/c1871de47df2a9597f13c7421980095b5d2d8b2c))


### Bug Fixes

* **describe:** route keys to search input, not global tab handler ([#203](https://github.com/janosmiko/lfk/issues/203)) ([#204](https://github.com/janosmiko/lfk/issues/204)) ([40ea18d](https://github.com/janosmiko/lfk/commit/40ea18de1aeb7ad4dc3c5ee29573da9688d7834d))
* **exec,browser:** make interactive shell + browser-open actions work on Windows ([#197](https://github.com/janosmiko/lfk/issues/197)) ([4954439](https://github.com/janosmiko/lfk/commit/4954439941e12860973643a0f6a91b36232a713e))
* **release:** skip Chocolatey publish until first version is moderated ([#201](https://github.com/janosmiko/lfk/issues/201)) ([0a5be5d](https://github.com/janosmiko/lfk/commit/0a5be5d8d3fa0082576a2d04f68bf42538c6b7cc))

## [0.11.0](https://github.com/janosmiko/lfk/compare/v0.10.4...v0.11.0) (2026-05-09)


### Features

* **k8s:** surface ephemeral containers in pod views ([#180](https://github.com/janosmiko/lfk/issues/180)) ([ac1a1c5](https://github.com/janosmiko/lfk/commit/ac1a1c54baf293482a7a29666336816838713332))
* **localcluster:** manage kind/k3d/minikube clusters from inside lfk ([#175](https://github.com/janosmiko/lfk/issues/175)) ([3c85fd9](https://github.com/janosmiko/lfk/commit/3c85fd9955f5fd88dc78efefd298cac11f0e6bf3))
* **release:** add AUR channel (lfk-bin) ([#174](https://github.com/janosmiko/lfk/issues/174)) ([c6df49d](https://github.com/janosmiko/lfk/commit/c6df49ddd153d0d15148e0dfa181f9440a19a702))
* **scheduler:** priority task queue with per-context dispatch ([#186](https://github.com/janosmiko/lfk/issues/186)) ([80e0ba1](https://github.com/janosmiko/lfk/commit/80e0ba13f5dc7d4a3609e3233002752c6686d8c6))
* traffic capture (kubectl-debug + kubeshark backends) ([#179](https://github.com/janosmiko/lfk/issues/179)) ([b51d64c](https://github.com/janosmiko/lfk/commit/b51d64c421a4001ff1cf1c53efc2792cceb41b3f))
* **viewers:** vim text-object selection (viw/vaw/viW/vaW) ([#185](https://github.com/janosmiko/lfk/issues/185)) ([7eb0aea](https://github.com/janosmiko/lfk/commit/7eb0aeaf2b5263e7c7fd630b3ac300b7e5140061))


### Bug Fixes

* **actions:** block delete keypress in containers view ([#181](https://github.com/janosmiko/lfk/issues/181)) ([584ff4f](https://github.com/janosmiko/lfk/commit/584ff4fd0bf50f3ba1c65b061469bcf741e09804))
* **nodeshell:** land on DiskPressure/MemoryPressure/PIDPressure nodes ([#177](https://github.com/janosmiko/lfk/issues/177)) ([eec8d02](https://github.com/janosmiko/lfk/commit/eec8d02a5426c1667f9d0a195bfa8765da1c98d2))
* stop infinite Loading spinner on permission errors ([#171](https://github.com/janosmiko/lfk/issues/171)) ([07c4c14](https://github.com/janosmiko/lfk/commit/07c4c14c90800144ad5999218fbfdf887069dfc9))
* **tabs:** refresh middle column on tab switch (stale-while-revalidate) ([#182](https://github.com/janosmiko/lfk/issues/182)) ([b84595e](https://github.com/janosmiko/lfk/commit/b84595e5718c663ed9b6096f4e0f6e3960f065d0))
* **ui:** clip pinned resource-usage footer no longer triggered by event count ([#178](https://github.com/janosmiko/lfk/issues/178)) ([54b6af0](https://github.com/janosmiko/lfk/commit/54b6af0cdf2c56192ec15bc6233c4489cdf3753f))

## [0.10.4](https://github.com/janosmiko/lfk/compare/v0.10.3...v0.10.4) (2026-05-06)


### Features

* **argocd:** add Sync Wave Timeline overlay ([#160](https://github.com/janosmiko/lfk/issues/160)) ([3784fc6](https://github.com/janosmiko/lfk/commit/3784fc6e3de25fa3774457f487b0c5840e01131f))
* **release:** add cloudsmith deb+rpm channel ([#163](https://github.com/janosmiko/lfk/issues/163)) ([8c50bec](https://github.com/janosmiko/lfk/commit/8c50bec5d2e7fb158d960543939b57b171b2915a))
* **release:** add scoop, winget, chocolatey channels ([#161](https://github.com/janosmiko/lfk/issues/161)) ([f4fe4a5](https://github.com/janosmiko/lfk/commit/f4fe4a576486b3c84e1949f773e6cfc3bad0ad84))

## [0.10.3](https://github.com/janosmiko/lfk/compare/v0.10.2...v0.10.3) (2026-05-06)


### Features

* **release:** foundation for new package-manager channels ([#159](https://github.com/janosmiko/lfk/issues/159)) ([0a4a353](https://github.com/janosmiko/lfk/commit/0a4a353a3e715b05a6b5b86e2a4183e26c9b21e9))
* **ui:** dim explorer behind overlays via dim_overlay option ([#99](https://github.com/janosmiko/lfk/issues/99)) ([df167f4](https://github.com/janosmiko/lfk/commit/df167f4bb20f65eda38b72ff80424755d8deb8d5))


### Bug Fixes

* **filter:** clear active filter preset on Esc ([#156](https://github.com/janosmiko/lfk/issues/156)) ([7b22dff](https://github.com/janosmiko/lfk/commit/7b22dff6d59d31d8484536abe445cd6774be45a8))
* **filter:** clear stale preview when filter preset matches zero items ([#157](https://github.com/janosmiko/lfk/issues/157)) ([5a8f28c](https://github.com/janosmiko/lfk/commit/5a8f28c79aa2579ab184a42b8184855f059469f9))
* **theme:** keep parent highlight readable on themes with near-text border ([b5fc86f](https://github.com/janosmiko/lfk/commit/b5fc86f4ea5b9abb0c1f54a10d356395a8f907a2))

## [0.10.2](https://github.com/janosmiko/lfk/compare/v0.10.1...v0.10.2) (2026-05-05)


### Bug Fixes

* **release:** rename cosign bundle to .sigstore for Scorecard ([#152](https://github.com/janosmiko/lfk/issues/152)) ([f12ce39](https://github.com/janosmiko/lfk/commit/f12ce3976ceae1fd6255d573de842647a687a1a4))

## [0.10.1](https://github.com/janosmiko/lfk/compare/v0.10.0...v0.10.1) (2026-05-05)


### Bug Fixes

* **release:** declare cosign bundle as signature artifact ([#150](https://github.com/janosmiko/lfk/issues/150)) ([1adf6ea](https://github.com/janosmiko/lfk/commit/1adf6eae58e26f23510291c0727e60cc8b24da60))

## [0.10.0](https://github.com/janosmiko/lfk/compare/v0.9.39...v0.10.0) (2026-05-05)


### ⚠ BREAKING CHANGES

* add multi-strategy right-sizing advisor overlay ([#148](https://github.com/janosmiko/lfk/issues/148))
* CrashLoopBackOff investigator overlay

### Features

* add multi-strategy right-sizing advisor overlay ([#148](https://github.com/janosmiko/lfk/issues/148)) ([5392610](https://github.com/janosmiko/lfk/commit/539261090646f1dc94c19dcd3c1b57eca1e7b1bb))
* CrashLoopBackOff investigator overlay ([93d310e](https://github.com/janosmiko/lfk/commit/93d310e8eb2b6547c8967749aec7a5e5a318f9ef))


### Bug Fixes

* **metrics:** stop ~1Hz column-order blink on PodInitializing rows ([0895f56](https://github.com/janosmiko/lfk/commit/0895f563667f9ae1cd25a08d2f0ac33a5a21f111))
* **metrics:** stop ~1Hz column-order blink on PodInitializing rows ([b1b53cf](https://github.com/janosmiko/lfk/commit/b1b53cf05b1bc8fbddd0188b80d9727f162d3f76))
* **release:** migrate cosign signing to Sigstore bundle output ([acc6284](https://github.com/janosmiko/lfk/commit/acc62843638f64d1ca38a762cd1cc05295bf43e2))
* **ui:** stop namespace and column-toggle overlays from shrinking on filter ([7965ecd](https://github.com/janosmiko/lfk/commit/7965ecd49b03690512fe2d3622a1062a3a74cec1))
* **ui:** stop selector overlays from shrinking on filter ([39d0ba2](https://github.com/janosmiko/lfk/commit/39d0ba2a5a5ff952dfc2dd6bcbc170e78ced4dfa))

## [0.9.39](https://github.com/janosmiko/lfk/compare/v0.9.38...v0.9.39) (2026-05-04)


### Features

* **clusters:** add per-cluster color coding with title-bar tint ([#124](https://github.com/janosmiko/lfk/issues/124)) ([65da3ac](https://github.com/janosmiko/lfk/commit/65da3ac010f4b84b4270dc4ce8662243a7171497))
* **editors:** revamp edit pane — bordered fields + non-shifting cursor ([55b322a](https://github.com/janosmiko/lfk/commit/55b322a33113fbff87ec296a51100e8b5fb41a4e))
* **editors:** wire `s` multi-select + Shift+Y format-copy on ConfigMap + Label editors ([44429de](https://github.com/janosmiko/lfk/commit/44429def06b4106787b14dcb5fc15d907c8741be))
* **editors:** wire `s` multi-select + Shift+Y format-copy on Secret editor ([f240d35](https://github.com/janosmiko/lfk/commit/f240d358e0417e787eaac477b9b3162d42a4b71d))
* **mouse:** click-to-drill, right-click action menu, overlay mouse ([8287ba0](https://github.com/janosmiko/lfk/commit/8287ba0b3fa5c50f462f063293c5915f3871a51c))
* **networking:** per-endpoint preview for Endpoints / EndpointSlices ([fb0201d](https://github.com/janosmiko/lfk/commit/fb0201d9513a19ff9c3ad429a0a331b5916f9af5))
* **networking:** Service preview rollup of backing EndpointSlices ([66a1e26](https://github.com/janosmiko/lfk/commit/66a1e26b74894eafd53b746012bd8eeb50586c40))
* **rbac:** reverse-RBAC "Who-Can" view, layered on the Can-I overlay ([7598c68](https://github.com/janosmiko/lfk/commit/7598c68501a0db5067614c8cbf35f3e91020d7fc))
* **resource-map:** traverse Pod refs with MissingRef detection ([a0517c3](https://github.com/janosmiko/lfk/commit/a0517c306964e02bfd8a3126688727ec4f96bf77))
* **viewers:** extend count-prefix to column / word / page / search motions ([64e9498](https://github.com/janosmiko/lfk/commit/64e9498894acd84684a57679b9387c364de36f56))
* **viewers:** match vim/nvim [count]&lt;C-d&gt;/&lt;C-u&gt; 'scroll' option semantics ([fd83a57](https://github.com/janosmiko/lfk/commit/fd83a57cd57c5bc950d507ad78ce7c38f0e00bec))


### Bug Fixes

* address CodeRabbit findings on PR [#122](https://github.com/janosmiko/lfk/issues/122) ([1a0a97c](https://github.com/janosmiko/lfk/commit/1a0a97cedafa22d2f623b1cb5091c73f2fd6d462))
* address second round of CodeRabbit findings on PR [#122](https://github.com/janosmiko/lfk/issues/122) ([3fffdd4](https://github.com/janosmiko/lfk/commit/3fffdd4ff336da514768307b131c79067e696252))
* clear stale pod metrics when metrics-server payload is empty ([0191775](https://github.com/janosmiko/lfk/commit/01917750e54e90baba1e1694c4dd301f8f39d50a))
* **editors:** address CodeRabbit review on PR [#134](https://github.com/janosmiko/lfk/issues/134) ([c18c1ce](https://github.com/janosmiko/lfk/commit/c18c1ce1debc0296747f146282803c7d9af3ff88))
* **editors:** ANSI leak in field labels + up/down nav + scroll-to-cursor ([b82144c](https://github.com/janosmiko/lfk/commit/b82144c9a22b0e647ba984d9aa66bcec76f440ce))
* **editors:** collapse long/multi-line values to a single visual cell ([97481d5](https://github.com/janosmiko/lfk/commit/97481d5aa6ebf98798a2e12caba335a081f409f9))
* **editors:** consistent key column + space-select + smart-y ([1302e3b](https://github.com/janosmiko/lfk/commit/1302e3b46cb8e31149f109080298faba9adb9c13))
* **editors:** ctrl+s under active filter no longer mutates wrong key ([c02de2b](https://github.com/janosmiko/lfk/commit/c02de2b35d6bf09ee4a538aa1bc5726ebbd01663))
* **editors:** cursor in edit pane lands at TextInput cursor pos + ([6ab3526](https://github.com/janosmiko/lfk/commit/6ab35266d90fa34f6a1d82b53d32ee91269d5b5c))
* **editors:** format picker no longer shrinks the table ([1e0a2eb](https://github.com/janosmiko/lfk/commit/1e0a2eb1d087a0e8b25fe39b5d48b501c498a50d))
* **editors:** inline edit mode for single-line values ([2cc3e53](https://github.com/janosmiko/lfk/commit/2cc3e53be4b80e377ff9903510d68b35b63d368b))
* **editors:** show multi-line values as multi-line during editing ([6c59ceb](https://github.com/janosmiko/lfk/commit/6c59ceb9c4d79ef96d72655fd250e5fca10b1e19))
* **editors:** sticky scroll + ctrl+u/d/f/b page keys + line-scoped ctrl+a/e ([1d8ba02](https://github.com/janosmiko/lfk/commit/1d8ba021ab7ba45db07bbc1a4ea91235b2f0e313))
* **help:** address lint and CodeRabbit review ([490fe6d](https://github.com/janosmiko/lfk/commit/490fe6daf363b8011508a08fea8f2ab321eaf70d))
* **mouse:** address CodeRabbit review on PR [#135](https://github.com/janosmiko/lfk/issues/135) ([ed9293b](https://github.com/janosmiko/lfk/commit/ed9293b2e3c9741e8afff240be802d253db74ad9))
* **nav:** preserve cursor on watch-tick discovery failure at LevelResourceTypes ([057f036](https://github.com/janosmiko/lfk/commit/057f036fb904e2df8494ebdf65b1cb65796745b1))
* **networking:** address coderabbit findings on Service endpoints rollup ([de75cb3](https://github.com/janosmiko/lfk/commit/de75cb31dfdfa87d1134ddc05ab9b64064680293))
* **networking:** always refetch Service endpoints; cache hid pod churn ([906b37f](https://github.com/janosmiko/lfk/commit/906b37fb80b547be611f75203e8546352acfea4e))
* **networking:** carry over Service rollup columns across watch-tick rebuilds ([e67c272](https://github.com/janosmiko/lfk/commit/e67c272da3f17da5b16b35bb2606dea8600882ed))
* **networking:** stale-while-revalidate Service endpoints to stop the flash ([24b6f95](https://github.com/janosmiko/lfk/commit/24b6f95e5b78806787693c3150883b57168f3472))
* **networking:** treat absent EndpointSlice conditions.ready as ready ([94ac9fc](https://github.com/janosmiko/lfk/commit/94ac9fc08c66a5691ff68df90b33691a49b926e9))
* **preview:** clear previewLoading when resource list arrives empty ([a781377](https://github.com/janosmiko/lfk/commit/a781377429081c8794ce7ccd20674b7f03cf031a))
* **preview:** DATA (N) counts keys not visual lines ([fa623d4](https://github.com/janosmiko/lfk/commit/fa623d4bbff9a39749a90060f2bb4d577faf6934))
* **quit:** cancel in-flight API requests so quit doesn't hang on dead clusters ([b0479b8](https://github.com/janosmiko/lfk/commit/b0479b8978a32387c15452cf0ff077e9e005664c))
* **rbac:** address remaining coderabbit findings on Who-Can ([8acdd11](https://github.com/janosmiko/lfk/commit/8acdd11965492c7f1d1f6a328ad7aa1e514678b4))
* **resource-map:** fall back to nav.Namespace at LevelContainers ([13033f3](https://github.com/janosmiko/lfk/commit/13033f3195f66bb5c521f6c2aa532b467681dcad))
* **resource-map:** show Pod's tree when M is pressed at LevelContainers ([36b84fa](https://github.com/janosmiko/lfk/commit/36b84fa7f81ee5dd57e386d407997f7d58f349a8))
* **tabs:** persist right-pane footers per tab so metrics don't bleed ([1a4fa9d](https://github.com/janosmiko/lfk/commit/1a4fa9d20f3b225e3e727dd0dd72269928ad1b0f))
* **viewers:** clear diff digit buffer on visual mode entry ([bac6309](https://github.com/janosmiko/lfk/commit/bac63095c5ae74baff9dfd0a409362ae63ae48ec))
* **viewers:** round half-page step before scaling by count ([316a91d](https://github.com/janosmiko/lfk/commit/316a91dffd2446afde5c21d69f1e9b0147682c38))
* **viewers:** scale yaml page motions by viewport, not raw m.height ([67dd610](https://github.com/janosmiko/lfk/commit/67dd6102ad5473f136723107fc1eed9690e4206e))

## [0.9.38](https://github.com/janosmiko/lfk/compare/v0.9.37...v0.9.38) (2026-05-02)


### Features

* **logs:** persistent search history with Up/Down recall ([58d6b08](https://github.com/janosmiko/lfk/commit/58d6b08693996fb7d292b471de5159d424133119))
* **logs:** persistent search history with Up/Down recall in log viewer ([cc70537](https://github.com/janosmiko/lfk/commit/cc7053710e8f338aa7bbc70b66ae32d8f4f5c5d6))


### Bug Fixes

* **history:** preserve draft on edit-after-recall via leaveBrowse() ([321a4bc](https://github.com/janosmiko/lfk/commit/321a4bc4e75c62bc3a3110996fe8a3f687600352))
* **history:** tighten file perms and leaveBrowse on paste ([0678df4](https://github.com/janosmiko/lfk/commit/0678df45075c3b136359a42db236ff1ee2f673e7))
* **logs:** handle Ctrl+U (delete-line) in log viewer search input ([cd1a049](https://github.com/janosmiko/lfk/commit/cd1a0495721989cb574777ccd6876242870db6f0))
* **logs:** scope log-search backspace reset() inside len-guard ([6608c14](https://github.com/janosmiko/lfk/commit/6608c14640902b22495c82a03709c2410978cab4))

## [0.9.37](https://github.com/janosmiko/lfk/compare/v0.9.36...v0.9.37) (2026-05-02)


### Features

* **ui:** support count-prefixed motion (Nj/Nk) in read-only viewers ([1068839](https://github.com/janosmiko/lfk/commit/10688392fb6bf4b49d6a8b20bd3ada5ad1b3335f))
* **ui:** support count-prefixed yank (Ny) in read-only viewers ([28782d1](https://github.com/janosmiko/lfk/commit/28782d1e9064fe83339192d8f406e7d93b516c9a))


### Bug Fixes

* **nav:** clear filter state when navigating to parent ([9977274](https://github.com/janosmiko/lfk/commit/99772748fba84eb64889d63bd417dd0f597cf007))
* **ui:** keep "/" search highlight from corrupting SGR codes ([8383f6b](https://github.com/janosmiko/lfk/commit/8383f6bed5fb9840f331b3510af117911c5dc4b9))

## [0.9.36](https://github.com/janosmiko/lfk/compare/v0.9.35...v0.9.36) (2026-04-30)


### Features

* **k8s:** cache resource lists via shared informer (closes [#86](https://github.com/janosmiko/lfk/issues/86)) ([c8578cc](https://github.com/janosmiko/lfk/commit/c8578cc3fb4b7997a780741ddbe14e54b6e807fe))
* **ui:** advertise y/n alongside Enter/Esc for confirm dialogs ([4f95b5c](https://github.com/janosmiko/lfk/commit/4f95b5cf4140371c1cfa00fb4cf78b1faca9c779))
* **ui:** pin info chips far-right + entry-aware keymap fit ([#101](https://github.com/janosmiko/lfk/issues/101)) ([1163c7a](https://github.com/janosmiko/lfk/commit/1163c7a9ea928e484d75b500f7db3029035ac029))


### Bug Fixes

* **app:** keep silent ns refresh from clobbering an open overlay ([73d5ba2](https://github.com/janosmiko/lfk/commit/73d5ba2dd8483ddbc2980559ca1a06306097ea5f))
* **app:** make node shell work on SELinux-enforcing immutable distros ([4477e9c](https://github.com/janosmiko/lfk/commit/4477e9cc516d269dc23093c9af8c957a9c2a1689))
* **k8s,ui:** harden informer cache wiring + config parsing ([ddd596e](https://github.com/janosmiko/lfk/commit/ddd596eb2f21f86d5d863aab50a4dc9c803ac60b))
* **ui:** center quit overlay text and unify confirm-hint convention ([e200ffe](https://github.com/janosmiko/lfk/commit/e200ffe6cb0d140a749d695de86420dca72e5c79))
* **ui:** drop dangling CONTRIBUTING.md refs and tighten confirm-hint test ([315e04d](https://github.com/janosmiko/lfk/commit/315e04d07c0dbca699987f4db6ad042dfa064e59))
* **ui:** invalidate middle-column row cache on theme change ([01ca28f](https://github.com/janosmiko/lfk/commit/01ca28f6963e6c0cffc9203c79a44916d67532a4))
* **ui:** widen Quick Filters overlay and clean up selected row ([86afe1a](https://github.com/janosmiko/lfk/commit/86afe1acb04a3e2bb7e0fee670c931e999211a7c))


### Performance Improvements

* **app:** seed namespace selector overlay from existing cache ([d8090b4](https://github.com/janosmiko/lfk/commit/d8090b42ab45cf6f485f84957d8907cd6f136a75))

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
