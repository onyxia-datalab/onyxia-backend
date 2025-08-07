# Changelog

## [0.1.12](https://github.com/onyxia-datalab/onyxia-backend/compare/onboarding-v0.1.11...onboarding-v0.1.12) (2025-08-07)


### Features

* add chi first route ([#8](https://github.com/onyxia-datalab/onyxia-backend/issues/8)) ([72687e4](https://github.com/onyxia-datalab/onyxia-backend/commit/72687e4fe54c4bafa4dba8d5fe1e9dec02c1e6ea))
* add contextPath ([#65](https://github.com/onyxia-datalab/onyxia-backend/issues/65)) ([b8efba8](https://github.com/onyxia-datalab/onyxia-backend/commit/b8efba8a9b3f66d0e47bdd50b29fdd067352a1b7))
* add helm chart and oci push action ([#48](https://github.com/onyxia-datalab/onyxia-backend/issues/48)) ([08bdcf7](https://github.com/onyxia-datalab/onyxia-backend/commit/08bdcf7634ffdc650e0c4c5634064107801050e1))
* Add namespaceLabel on creation and update ([#74](https://github.com/onyxia-datalab/onyxia-backend/issues/74)) ([c1003fa](https://github.com/onyxia-datalab/onyxia-backend/commit/c1003fa29e3449ca16eeed949858a08c4e337ac6))
* add qemu and buildx setup to reduce docker actions time ([#44](https://github.com/onyxia-datalab/onyxia-backend/issues/44)) ([ff58332](https://github.com/onyxia-datalab/onyxia-backend/commit/ff583322e8894686e32172d1a98b84310da468e7))
* add quotas support for namespace, split files  ([#25](https://github.com/onyxia-datalab/onyxia-backend/issues/25)) ([4302685](https://github.com/onyxia-datalab/onyxia-backend/commit/43026854c43ef62fb2e73434db7e3319e24283a6))
* add support of env variables ([#19](https://github.com/onyxia-datalab/onyxia-backend/issues/19)) ([1124d90](https://github.com/onyxia-datalab/onyxia-backend/commit/1124d90d743fc3edd967bb1c69cac56175c9fdb4))
* add username, groups and roles in log ([87c9580](https://github.com/onyxia-datalab/onyxia-backend/commit/87c958077d9be72b32069f61b78aa3daae150024))
* clean archi with ogen ([#21](https://github.com/onyxia-datalab/onyxia-backend/issues/21)) ([3c6dc0b](https://github.com/onyxia-datalab/onyxia-backend/commit/3c6dc0b6a8fb31e9de8d0f8906d949be0bffaf4c))
* Implement role-based quotas and validate group onboarding rights ([#33](https://github.com/onyxia-datalab/onyxia-backend/issues/33)) ([af5ea45](https://github.com/onyxia-datalab/onyxia-backend/commit/af5ea45bfe5193446d3a14d8060fb7052888d3a3))
* Improve default env handling with embedded config ([#34](https://github.com/onyxia-datalab/onyxia-backend/issues/34)) ([b75b0ef](https://github.com/onyxia-datalab/onyxia-backend/commit/b75b0ef655af6ba8cdee301e832d4ae593208b21))
* makefile and adapt CI ([#36](https://github.com/onyxia-datalab/onyxia-backend/issues/36)) ([5e024bc](https://github.com/onyxia-datalab/onyxia-backend/commit/5e024bc46b40ed62e29c582f7062e91ab1414709))
* role base quotas for user and refacto ctx ([#35](https://github.com/onyxia-datalab/onyxia-backend/issues/35)) ([6eef813](https://github.com/onyxia-datalab/onyxia-backend/commit/6eef8131a56d1b2a2fcacd87ebe233b345a286bd))
* setup renovate ([#4](https://github.com/onyxia-datalab/onyxia-backend/issues/4)) ([ed63151](https://github.com/onyxia-datalab/onyxia-backend/commit/ed631516cf12ad60f8389279e32b7e99075f8462))
* trigger release ([60d2723](https://github.com/onyxia-datalab/onyxia-backend/commit/60d272394706f0a8efb8047ea44d9668f8df5d5e))
* trigger release ([2262282](https://github.com/onyxia-datalab/onyxia-backend/commit/2262282ade0fd222256f3c765d8d8da1cd544d2f))
* trigger release ([5a634fd](https://github.com/onyxia-datalab/onyxia-backend/commit/5a634fdda8d4e473923feb4d92767fe6c6635e2c))


### Bug Fixes

* add EnvKeyReplacer to support environment variables with nested … ([#67](https://github.com/onyxia-datalab/onyxia-backend/issues/67)) ([255dcbe](https://github.com/onyxia-datalab/onyxia-backend/commit/255dcbe4aac93a1ebf07a3301fdeecd7f6c07d1d))
* **ci:** make tags available in ci so docker tags are correct ([#38](https://github.com/onyxia-datalab/onyxia-backend/issues/38)) ([80d6e17](https://github.com/onyxia-datalab/onyxia-backend/commit/80d6e17df5f22cad962a5150859ed2439480b985))
* cors issue due to onyxia-region header ([#75](https://github.com/onyxia-datalab/onyxia-backend/issues/75)) ([f729ca5](https://github.com/onyxia-datalab/onyxia-backend/commit/f729ca59c9be19b179d5ccf858e34baa29dbd654))
* **deps:** update go minor and patch updates ([#63](https://github.com/onyxia-datalab/onyxia-backend/issues/63)) ([d688f1f](https://github.com/onyxia-datalab/onyxia-backend/commit/d688f1f44ded2d7e7c95913ded8fedf9323baf3e))
* **deps:** update go minor and patch updates to v0.33.3 ([#71](https://github.com/onyxia-datalab/onyxia-backend/issues/71)) ([3e8a36d](https://github.com/onyxia-datalab/onyxia-backend/commit/3e8a36d606cdab2a5d05d6b70b2e4853c86ce3f6))
* **deps:** update kubernetes packages to v0.32.2 ([#29](https://github.com/onyxia-datalab/onyxia-backend/issues/29)) ([9983a34](https://github.com/onyxia-datalab/onyxia-backend/commit/9983a34e83b713bff50740d4c0324d9dfe802848))
* **deps:** update module github.com/coreos/go-oidc/v3 to v3.15.0 ([#73](https://github.com/onyxia-datalab/onyxia-backend/issues/73)) ([577452b](https://github.com/onyxia-datalab/onyxia-backend/commit/577452b22ac549f95217fd0734ec4b7dc8fe0511))
* **deps:** update module github.com/go-chi/chi/v5 to v5.2.2 [security] ([#64](https://github.com/onyxia-datalab/onyxia-backend/issues/64)) ([8db1356](https://github.com/onyxia-datalab/onyxia-backend/commit/8db13567b7b208b82b7b8c6b8c818b77c6ce1525))
* **deps:** update module github.com/ogen-go/ogen to v1.10.0 ([#24](https://github.com/onyxia-datalab/onyxia-backend/issues/24)) ([e26cf7f](https://github.com/onyxia-datalab/onyxia-backend/commit/e26cf7f293d0b93b6cdb38289a2e5e5659874410))
* **Dockerfile:** app path & go compile option for distroless image ([#61](https://github.com/onyxia-datalab/onyxia-backend/issues/61)) ([b5c90ab](https://github.com/onyxia-datalab/onyxia-backend/commit/b5c90abf641a43942f867ee6f80a35460c2d8d09))
* error introduced by [#8](https://github.com/onyxia-datalab/onyxia-backend/issues/8) ([6d73b2c](https://github.com/onyxia-datalab/onyxia-backend/commit/6d73b2c2f5331c9a3341055dff04b892f7a7de14))
* humbly fixing the linting error ([#13](https://github.com/onyxia-datalab/onyxia-backend/issues/13)) ([efe41f8](https://github.com/onyxia-datalab/onyxia-backend/commit/efe41f849357a74f9fe9f448f8dbb26099f23afa))
* oidc groups and roles extractions ([ab65d1f](https://github.com/onyxia-datalab/onyxia-backend/commit/ab65d1f8d682e535fca7cafc610318ab6ff5af1b))
* oidc_test ([2fe8adc](https://github.com/onyxia-datalab/onyxia-backend/commit/2fe8adc7a4107a38bc1cfc2c40ddac62c59519d2))
* **oidc:** support audience claim as []interface{} in JWT token ([#69](https://github.com/onyxia-datalab/onyxia-backend/issues/69)) ([6ab4902](https://github.com/onyxia-datalab/onyxia-backend/commit/6ab490257ad1be70061d6d9ef04e6d5c62089058))
* renovate use conventional commits ([#7](https://github.com/onyxia-datalab/onyxia-backend/issues/7)) ([5163e27](https://github.com/onyxia-datalab/onyxia-backend/commit/5163e275988b34fa4d802f046d119754c4512a94))
* **test:** ignore renovate PRs ([6b68e06](https://github.com/onyxia-datalab/onyxia-backend/commit/6b68e063ce26312b29d2dd27ae125d44a3ed97d3))

## [0.1.11](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.10...v0.1.11) (2025-08-06)


### Bug Fixes

* cors issue due to onyxia-region header ([#75](https://github.com/onyxia-datalab/onyxia-onboarding/issues/75)) ([136fe46](https://github.com/onyxia-datalab/onyxia-onboarding/commit/136fe4616ccf2176a4a1916f1c03f5ceca9fa6fe))
* **deps:** update module github.com/coreos/go-oidc/v3 to v3.15.0 ([#73](https://github.com/onyxia-datalab/onyxia-onboarding/issues/73)) ([4620649](https://github.com/onyxia-datalab/onyxia-onboarding/commit/4620649a396fa0a8100778589ee643aaa92d492d))

## [0.1.10](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.9...v0.1.10) (2025-07-31)


### Features

* Add namespaceLabel on creation and update ([#74](https://github.com/onyxia-datalab/onyxia-onboarding/issues/74)) ([d09784f](https://github.com/onyxia-datalab/onyxia-onboarding/commit/d09784f2f0544fd6765953a7a2ba4854115c1f3b))


### Bug Fixes

* **deps:** update go minor and patch updates to v0.33.3 ([#71](https://github.com/onyxia-datalab/onyxia-onboarding/issues/71)) ([2a248c9](https://github.com/onyxia-datalab/onyxia-onboarding/commit/2a248c91f3e91200e9406f1d0749090b207248df))

## [0.1.9](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.8...v0.1.9) (2025-07-07)


### Bug Fixes

* **oidc:** support audience claim as []interface{} in JWT token ([#69](https://github.com/onyxia-datalab/onyxia-onboarding/issues/69)) ([d1bac73](https://github.com/onyxia-datalab/onyxia-onboarding/commit/d1bac730b11d0ac5225c12d451431c82d08efbec))

## [0.1.8](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.7...v0.1.8) (2025-07-07)


### Bug Fixes

* add EnvKeyReplacer to support environment variables with nested … ([#67](https://github.com/onyxia-datalab/onyxia-onboarding/issues/67)) ([a29352a](https://github.com/onyxia-datalab/onyxia-onboarding/commit/a29352aa4d355e723e163b4a2318797ad647958e))

## [0.1.7](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.6...v0.1.7) (2025-07-03)


### Features

* add contextPath ([#65](https://github.com/onyxia-datalab/onyxia-onboarding/issues/65)) ([2359a91](https://github.com/onyxia-datalab/onyxia-onboarding/commit/2359a91c2031a435bc003479decd1bb4213fd93b))


### Bug Fixes

* **deps:** update go minor and patch updates ([#63](https://github.com/onyxia-datalab/onyxia-onboarding/issues/63)) ([add1a05](https://github.com/onyxia-datalab/onyxia-onboarding/commit/add1a05661e3024481f6c56e531aee438568b1b3))
* **deps:** update module github.com/go-chi/chi/v5 to v5.2.2 [security] ([#64](https://github.com/onyxia-datalab/onyxia-onboarding/issues/64)) ([ef95327](https://github.com/onyxia-datalab/onyxia-onboarding/commit/ef95327a6cba6102d44d930f8f4b8df382d4d997))

## [0.1.6](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.5...v0.1.6) (2025-06-19)


### Features

* add helm chart and oci push action ([#48](https://github.com/onyxia-datalab/onyxia-onboarding/issues/48)) ([40547f0](https://github.com/onyxia-datalab/onyxia-onboarding/commit/40547f04f8125991ef3865529e4e15d7890b383e))


### Bug Fixes

* **Dockerfile:** app path & go compile option for distroless image ([#61](https://github.com/onyxia-datalab/onyxia-onboarding/issues/61)) ([3de4f94](https://github.com/onyxia-datalab/onyxia-onboarding/commit/3de4f945035fd8692c93b12e2196fd41d7a10c25))

## [0.1.5](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.4...v0.1.5) (2025-02-27)


### Features

* trigger release ([07ecbac](https://github.com/onyxia-datalab/onyxia-onboarding/commit/07ecbac285eb029c2e64d36946903a746d4faa77))

## [0.1.4](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.3...v0.1.4) (2025-02-27)


### Features

* trigger release ([17c9b7e](https://github.com/onyxia-datalab/onyxia-onboarding/commit/17c9b7e6dde1a184bcf62fc86be3668f6e01ccf4))

## [0.1.3](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.2...v0.1.3) (2025-02-27)


### Features

* add qemu and buildx setup to reduce docker actions time ([#44](https://github.com/onyxia-datalab/onyxia-onboarding/issues/44)) ([e56c3b6](https://github.com/onyxia-datalab/onyxia-onboarding/commit/e56c3b63e32193d9256b329396a731c3eb94cc4d))

## [0.1.2](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.1...v0.1.2) (2025-02-27)


### Features

* trigger release ([ad028b6](https://github.com/onyxia-datalab/onyxia-onboarding/commit/ad028b618ff25dc1b0dda5649a2e0cca17609691))

## [0.1.1](https://github.com/onyxia-datalab/onyxia-onboarding/compare/v0.1.0...v0.1.1) (2025-02-25)


### Bug Fixes

* **ci:** make tags available in ci so docker tags are correct ([#38](https://github.com/onyxia-datalab/onyxia-onboarding/issues/38)) ([dfaa2dc](https://github.com/onyxia-datalab/onyxia-onboarding/commit/dfaa2dc9cbd85668da5944ba506dcf50588e0949))

## 0.1.0 (2025-02-25)


### Features

* add chi first route ([#8](https://github.com/onyxia-datalab/onyxia-onboarding/issues/8)) ([6e2af0a](https://github.com/onyxia-datalab/onyxia-onboarding/commit/6e2af0ad987a564890880b42bb0b6f076d3802f8))
* add quotas support for namespace, split files  ([#25](https://github.com/onyxia-datalab/onyxia-onboarding/issues/25)) ([0fa9e89](https://github.com/onyxia-datalab/onyxia-onboarding/commit/0fa9e899738c5bf04d891132a16e50fbec09ded6))
* add support of env variables ([#19](https://github.com/onyxia-datalab/onyxia-onboarding/issues/19)) ([37ffbd4](https://github.com/onyxia-datalab/onyxia-onboarding/commit/37ffbd4469e0f102bd9f92efed69fcb9df0425ef))
* add username, groups and roles in log ([2b0b5cc](https://github.com/onyxia-datalab/onyxia-onboarding/commit/2b0b5cc2f76a1d819bdf81b665a25b6f366d3521))
* clean archi with ogen ([#21](https://github.com/onyxia-datalab/onyxia-onboarding/issues/21)) ([a1cb014](https://github.com/onyxia-datalab/onyxia-onboarding/commit/a1cb0140b922bb767405a409a8b48fde38795221))
* Implement role-based quotas and validate group onboarding rights ([#33](https://github.com/onyxia-datalab/onyxia-onboarding/issues/33)) ([d61ad17](https://github.com/onyxia-datalab/onyxia-onboarding/commit/d61ad171cc9e96af007554e4be9ce8efb8eb81d5))
* Improve default env handling with embedded config ([#34](https://github.com/onyxia-datalab/onyxia-onboarding/issues/34)) ([ddc79b2](https://github.com/onyxia-datalab/onyxia-onboarding/commit/ddc79b22025af30969aeef1c3b0da1cd7ae4a0e8))
* makefile and adapt CI ([#36](https://github.com/onyxia-datalab/onyxia-onboarding/issues/36)) ([4cdfcc4](https://github.com/onyxia-datalab/onyxia-onboarding/commit/4cdfcc4e9d3984b7e9a04691f5c7887c4eaaacba))
* role base quotas for user and refacto ctx ([#35](https://github.com/onyxia-datalab/onyxia-onboarding/issues/35)) ([b5bca29](https://github.com/onyxia-datalab/onyxia-onboarding/commit/b5bca29ddbf3be27d64cd04dcd4211a661b4256a))
* setup renovate ([#4](https://github.com/onyxia-datalab/onyxia-onboarding/issues/4)) ([96859c4](https://github.com/onyxia-datalab/onyxia-onboarding/commit/96859c441696bd88745ba420fb20a0f9770621f6))


### Bug Fixes

* **deps:** update kubernetes packages to v0.32.2 ([#29](https://github.com/onyxia-datalab/onyxia-onboarding/issues/29)) ([5c6a47f](https://github.com/onyxia-datalab/onyxia-onboarding/commit/5c6a47fba4a9689ee863216ac77cd6d7594fc2ad))
* **deps:** update module github.com/ogen-go/ogen to v1.10.0 ([#24](https://github.com/onyxia-datalab/onyxia-onboarding/issues/24)) ([963aaef](https://github.com/onyxia-datalab/onyxia-onboarding/commit/963aaef99ad611c33f1e77017491f2b58131019f))
* error introduced by [#8](https://github.com/onyxia-datalab/onyxia-onboarding/issues/8) ([cb53a31](https://github.com/onyxia-datalab/onyxia-onboarding/commit/cb53a310dc53ecaf22cdf3986349c39fd7ebd677))
* humbly fixing the linting error ([#13](https://github.com/onyxia-datalab/onyxia-onboarding/issues/13)) ([f9b9d24](https://github.com/onyxia-datalab/onyxia-onboarding/commit/f9b9d2409397d76b83d552f989b8f1ebbb3420aa))
* oidc groups and roles extractions ([ab72ac2](https://github.com/onyxia-datalab/onyxia-onboarding/commit/ab72ac297bd44aa68e79939d89de760879b83de1))
* oidc_test ([7c0f35e](https://github.com/onyxia-datalab/onyxia-onboarding/commit/7c0f35ee03b9485e34fff5c1e2d670b27f1c8d44))
* renovate use conventional commits ([#7](https://github.com/onyxia-datalab/onyxia-onboarding/issues/7)) ([456e7b1](https://github.com/onyxia-datalab/onyxia-onboarding/commit/456e7b112aaa7e37b0785c96847780cc43406e05))
* **test:** ignore renovate PRs ([69083bc](https://github.com/onyxia-datalab/onyxia-onboarding/commit/69083bc6048b96b58cea2d06af0185698a1add1a))
