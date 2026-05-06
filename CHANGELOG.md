# Changelog

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
