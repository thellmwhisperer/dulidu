# Changelog

## [0.1.12](https://github.com/thellmwhisperer/dulidu/compare/v0.1.11...v0.1.12) (2026-10-21)


### Bug Fixes

* **pipeline:** generate screen-by-screen design briefs ([#124](https://github.com/thellmwhisperer/dulidu/issues/124)) ([c9427cd](https://github.com/thellmwhisperer/dulidu/commit/c9427cd1002bfb9e64b24ec15b2beefd2ea17966))

## [0.1.11](https://github.com/thellmwhisperer/dulidu/compare/v0.1.10...v0.1.11) (2026-10-20)


### Bug Fixes

* Cleanup: dulidu.yaml keys nobody reads, doctor without Go and with pdftotext, tolerant reading of fenced answers ([4a1a4bc](https://github.com/thellmwhisperer/dulidu/commit/4a1a4bc4fadabfd251df00d3f8d64e1ad8a1ca28))

## [0.1.10](https://github.com/thellmwhisperer/dulidu/compare/v0.1.9...v0.1.10) (2026-10-20)


### Bug Fixes

* Stories and tickets like the prototype: size and use cases back, each ticket filled from its own story ([6018c21](https://github.com/thellmwhisperer/dulidu/commit/6018c21b84c361afcae63dbf09555db3f1d70641))

## [0.1.9](https://github.com/thellmwhisperer/dulidu/compare/v0.1.8...v0.1.9) (2026-10-20)


### Bug Fixes

* **tui:** show readable agent output in terminal views ([#117](https://github.com/thellmwhisperer/dulidu/issues/117)) ([9cbdbf7](https://github.com/thellmwhisperer/dulidu/commit/9cbdbf701f1cb6f96f388a300fc6e90d3807c968))

## [0.1.8](https://github.com/thellmwhisperer/dulidu/compare/v0.1.7...v0.1.8) (2026-10-20)


### Bug Fixes

* Analysis and spec.md like the prototype: objects with ids, same context per list, named research sections ([e2a54f3](https://github.com/thellmwhisperer/dulidu/commit/e2a54f3cda8664208f90753ec71fa49d75e0cf06))
* Analysis keeps the model's objects and ids; epics and tickets use those ids ([31f42ba](https://github.com/thellmwhisperer/dulidu/commit/31f42ba1b16cf707dcd3d2ac443e5f96fd45dc99))
* dulidu runs still lists run folders written before the analysis objects ([24a3a98](https://github.com/thellmwhisperer/dulidu/commit/24a3a98692c886451c1facdd6ff34e4d95f472b3))

## [0.1.7](https://github.com/thellmwhisperer/dulidu/compare/v0.1.6...v0.1.7) (2026-10-20)


### Bug Fixes

* dulidu attach on a real terminal answers q, tab and the scroll keys ([942c9d5](https://github.com/thellmwhisperer/dulidu/commit/942c9d5146c60016b80f52733a4cef844867186e))

## [0.1.6](https://github.com/thellmwhisperer/dulidu/compare/v0.1.5...v0.1.6) (2026-10-20)


### Bug Fixes

* A video over the frame budget keeps its first part, with a note, instead of stopping the run ([1fe7d44](https://github.com/thellmwhisperer/dulidu/commit/1fe7d44a000aca0f1366cacf3b871a79c2ebde1b))
* Analysis lists may be empty, and the analysis-failure test fails by the agent instead ([07ebf5c](https://github.com/thellmwhisperer/dulidu/commit/07ebf5ccced849d85bb4b6f3bb11038ed69d1bc3))
* Epic dependencies: drop an unknown one or the edge that closes a cycle, with a note ([d8c46cc](https://github.com/thellmwhisperer/dulidu/commit/d8c46cce456dbe433fbcdf66386005272e7aa414))
* Epics trim to the cap instead of failing: first N epics, first N stories per epic ([c413c6e](https://github.com/thellmwhisperer/dulidu/commit/c413c6e6faafc26eca0855a31a84405e4f2f17cb))
* Plain-text reader: a list item wrapped over several lines keeps all of it ([f2d1f6b](https://github.com/thellmwhisperer/dulidu/commit/f2d1f6b9d2382c35f7277a84f4551ffacd7219fd))
* **release:** authenticate the answers job's GitHub API lookup ([c743512](https://github.com/thellmwhisperer/dulidu/commit/c743512d549423c767634abe178116d82010d879)), closes [#83](https://github.com/thellmwhisperer/dulidu/issues/83)
* Stories need only id and title; an unknown functionality id is dropped with a note ([84ae904](https://github.com/thellmwhisperer/dulidu/commit/84ae9045626a414af9523dac9fdf3a7398bc45bf))
* Uncovered must functionalities: a note in the epics step, not a failure ([673dd1e](https://github.com/thellmwhisperer/dulidu/commit/673dd1e5244fab0611be659aab9e63f1f22d5ee9))
* Unreadable attachments leave a note in contexto.md instead of stopping the run ([b5eb044](https://github.com/thellmwhisperer/dulidu/commit/b5eb04494d4d4d2700fdf5c172f95ef6bc2a6899))

## [0.1.5](https://github.com/thellmwhisperer/dulidu/compare/v0.1.4...v0.1.5) (2026-10-20)


### Features

* Epics read plain text: the whole run without JSON from the model ([be0bf38](https://github.com/thellmwhisperer/dulidu/commit/be0bf38993ec20a257b9610da7b12da3bc272bf1))

## [0.1.4](https://github.com/thellmwhisperer/dulidu/compare/v0.1.3...v0.1.4) (2026-10-20)


### Features

* Analysis reads plain text: one form per list ([59b41eb](https://github.com/thellmwhisperer/dulidu/commit/59b41ebdc103f1498a3b8f0b706a3ed38734c01b))

## [0.1.3](https://github.com/thellmwhisperer/dulidu/compare/v0.1.2...v0.1.3) (2026-10-20)


### Features

* Research reads plain text: prose and a Sources list ([5de259d](https://github.com/thellmwhisperer/dulidu/commit/5de259d165eca9543a4d1749cd1938d4810bc0b1))

## [0.1.2](https://github.com/thellmwhisperer/dulidu/compare/v0.1.1...v0.1.2) (2026-10-20)


### Features

* Triage reads plain text: a shared reader for the model's answers ([0ebe2d9](https://github.com/thellmwhisperer/dulidu/commit/0ebe2d9632300346f05506e52977fa16faf3e366))

## [0.1.1](https://github.com/thellmwhisperer/dulidu/compare/v0.1.0...v0.1.1) (2026-10-20)


### Features

* Add a README paragraph for private-clone installs and label the curl line as the public-repository form ([401a878](https://github.com/thellmwhisperer/dulidu/commit/401a878592b0b04425b68fc8c844db281be2b84b))


### Bug Fixes

* Enable Codex research web search through exec configuration ([5618fd4](https://github.com/thellmwhisperer/dulidu/commit/5618fd41115b3eca45c7973b3863992474b5303d))
* repair contract round 1 ([b15ce6e](https://github.com/thellmwhisperer/dulidu/commit/b15ce6e5daad4ab2429e431e241b426c573392f5))
* repair contract round 2 ([a11e4e7](https://github.com/thellmwhisperer/dulidu/commit/a11e4e72b780f33d6fcc4ac2de5bddec715917ce))
* Research asks the five dimensions every run; triage stops choosing them ([f5ce372](https://github.com/thellmwhisperer/dulidu/commit/f5ce372d4fcab3eb7398a4ac495008d074c59fa6))
* Research searches the web: claude gets WebSearch and WebFetch, the prompts say where to look ([4b926eb](https://github.com/thellmwhisperer/dulidu/commit/4b926eb956f2cb1515400718f22c7faacc6723bb))
* Research web switch for the other harnesses: each CLI's flag or web off in the note ([c997c3f](https://github.com/thellmwhisperer/dulidu/commit/c997c3f04aad1d8c4113d24f7c87f3e63889150a))
* Run never stops at triage: no waiting_answers, no --resume or --answers ([7b70c3b](https://github.com/thellmwhisperer/dulidu/commit/7b70c3b9d3e07c144fe8ef7def1cfe4a476e708f))
* Screen like the reference: pipeline box left, agent box right, scroll ([4a63f9b](https://github.com/thellmwhisperer/dulidu/commit/4a63f9bd46231c600bd436fc2cd1b5f1c4bceea5))
* Triage only classifies: complexity caps the epics, a failed triage is moderate ([a9d3fd9](https://github.com/thellmwhisperer/dulidu/commit/a9d3fd9ebccdde12204f64a4f5bcb3dc0974d814))
