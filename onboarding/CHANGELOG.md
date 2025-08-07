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

## Changelog

> For changes prior to v0.1.12, see the [onyxia-onboarding changelog](https://github.com/onyxia-datalab/onyxia-onboarding/blob/main/CHANGELOG.md).
