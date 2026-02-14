# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.1.0](https://github.com/bissquit/incident-garden/compare/v2.0.0...v2.1.0) (2026-02-14)


### Features

* add Prometheus metrics ([cd8428a](https://github.com/bissquit/incident-garden/commit/cd8428a389a6f8a53c8970d31ff8a4d97753118e))
* implement HTTP Server Timeouts ([508d6bb](https://github.com/bissquit/incident-garden/commit/508d6bb63deaec0641de4fea09ede727a6090935))
* implement Request ID in logs ([6ad41c8](https://github.com/bissquit/incident-garden/commit/6ad41c8c640323353df14a56d3ca6c97bd8bd1dc))


### Bug Fixes

* add database connection retry with exponential backoff ([913332e](https://github.com/bissquit/incident-garden/commit/913332e292aff70b03aa8580f4bd3446fd896683))
* add missing database connection config in integration tests ([4078950](https://github.com/bissquit/incident-garden/commit/4078950867b66f9c862a86a91d7405433a56d9b9))

## [2.0.0](https://github.com/bissquit/incident-garden/compare/v1.6.2...v2.0.0) (2026-02-11)


### ⚠ BREAKING CHANGES

* **events:** Change CreateEventRequest to include service statuses ([#49](https://github.com/bissquit/incident-garden/issues/49))

### Features

* **catalog:** Add effective_status computation for services ([#47](https://github.com/bissquit/incident-garden/issues/47)) ([9867dd6](https://github.com/bissquit/incident-garden/commit/9867dd64d64abd1fa43cd45bc23b527b6a5162ea))
* **catalog:** Add endpoint to get events for a service ([#55](https://github.com/bissquit/incident-garden/issues/55)) ([370ac93](https://github.com/bissquit/incident-garden/commit/370ac93e0a15a852dba5d46fe1d0ccc2d67b1a6b))
* **catalog:** Add service status audit log ([#53](https://github.com/bissquit/incident-garden/issues/53)) ([81b996f](https://github.com/bissquit/incident-garden/commit/81b996fd55c9d8b2f8de54ae9eb79aaf464a890d))
* **catalog:** Add service_ids field to UpdateGroup endpoint ([#41](https://github.com/bissquit/incident-garden/issues/41)) ([672ed91](https://github.com/bissquit/incident-garden/commit/672ed91bacb5e3416478715d1a10f30dd25ccbea))
* **events:** Add service status tracking within events ([#44](https://github.com/bissquit/incident-garden/issues/44)) ([3f3bd9b](https://github.com/bissquit/incident-garden/commit/3f3bd9b0465d9b597f86f5881685210aa16128d9))
* **events:** Change CreateEventRequest to include service statuses ([#49](https://github.com/bissquit/incident-garden/issues/49)) ([ab9a5c2](https://github.com/bissquit/incident-garden/commit/ab9a5c2518b2cc005a99a2ed449f275ddfb5461e))
* **events:** Extend event updates with service status management ([#51](https://github.com/bissquit/incident-garden/issues/51)) ([0e410be](https://github.com/bissquit/incident-garden/commit/0e410be0b8072decf8fd629063625855fdb359a7))
* **events:** Restrict deletion to resolved events only ([#56](https://github.com/bissquit/incident-garden/issues/56)) ([711786b](https://github.com/bissquit/incident-garden/commit/711786bb6101560fd1569a95adee5954f06e6ed6))


### Bug Fixes

* remove default message in event update ([#59](https://github.com/bissquit/incident-garden/issues/59)) ([a97c806](https://github.com/bissquit/incident-garden/commit/a97c806fb39b96262b81a5d0280f8a9fcdf03430))

## [1.6.2](https://github.com/bissquit/incident-garden/compare/v1.6.1...v1.6.2) (2026-02-06)


### Bug Fixes

* dont permit to archive group with services ([#38](https://github.com/bissquit/incident-garden/issues/38)) ([f924cf2](https://github.com/bissquit/incident-garden/commit/f924cf29041245ff011b0613fc4a561964eb2fb2))

## [1.6.1](https://github.com/bissquit/incident-garden/compare/v1.6.0...v1.6.1) (2026-02-06)


### Bug Fixes

* revert Cache-Control headers ([b80664b](https://github.com/bissquit/incident-garden/commit/b80664bcbe760a1f3f94122aab36d6430bfd1428))

## [1.6.0](https://github.com/bissquit/incident-garden/compare/v1.5.0...v1.6.0) (2026-02-05)


### Features

* allow read operation for services, groups, events (history, updates, changes) ([28488b9](https://github.com/bissquit/incident-garden/commit/28488b9e71ca06347da8074aca6088616c4045c7))


### Bug Fixes

* add Cache-Control headers ([149aef1](https://github.com/bissquit/incident-garden/commit/149aef1986e9295ae58f2e9a2aaf2cc4f934dab3))

## [1.5.0](https://github.com/bissquit/incident-garden/compare/v1.4.1...v1.5.0) (2026-02-04)


### Features

* **events:** add batch grouping for service changes in events ([#27](https://github.com/bissquit/incident-garden/issues/27)) ([fcda025](https://github.com/bissquit/incident-garden/commit/fcda025fffceee530d1cc836e70fa6d11413cf3d))

## [1.4.1](https://github.com/bissquit/incident-garden/compare/v1.4.0...v1.4.1) (2026-02-01)


### Bug Fixes

* include X-CSRF-Token into Access-Control-Allow-Headers ([#24](https://github.com/bissquit/incident-garden/issues/24)) ([2951b86](https://github.com/bissquit/incident-garden/commit/2951b868ed6af4115a0c704779829c0d6bdddd01))

## [1.4.0](https://github.com/bissquit/incident-garden/compare/v1.3.0...v1.4.0) (2026-02-01)


### Features

* implement HTTP-only cookies ([#21](https://github.com/bissquit/incident-garden/issues/21)) ([c83cfad](https://github.com/bissquit/incident-garden/commit/c83cfada4a959fe87ba056c51bbf06be6c98f9d7))

## [1.3.0](https://github.com/bissquit/incident-garden/compare/v1.2.2...v1.3.0) (2026-01-30)


### Features

* add events with groups and history of changes ([d9bfa20](https://github.com/bissquit/incident-garden/commit/d9bfa205682e8193fea0b8ac1695e35720d4772e))
* add one service to many groups relation ([c6ac95a](https://github.com/bissquit/incident-garden/commit/c6ac95ac18baeb3b6aa811f2bb4bec03ae8ebe83))
* implement kin-openapi validation ([c0b1277](https://github.com/bissquit/incident-garden/commit/c0b127797e87c1ada8759501210936083587fe49))
* soft delete and archived_at field ([9b5b5c6](https://github.com/bissquit/incident-garden/commit/9b5b5c6355a0b100db10c492c9b7675aec462e25))


### Bug Fixes

* update openapi ([5119c5c](https://github.com/bissquit/incident-garden/commit/5119c5ccb913b4d1ab3b86b2fce8de09236236c8))

## [1.2.2](https://github.com/bissquit/incident-garden/compare/v1.2.1...v1.2.2) (2026-01-29)


### Bug Fixes

* **db:** add demo data to initial db state ([#15](https://github.com/bissquit/incident-garden/issues/15)) ([ed70bea](https://github.com/bissquit/incident-garden/commit/ed70beabff89bee28ac983d8c8efb8b5e3bb8b53))

## [1.2.1](https://github.com/bissquit/incident-garden/compare/v1.2.0...v1.2.1) (2026-01-28)


### Bug Fixes

* handle null response and return correct json ([62e1da4](https://github.com/bissquit/incident-garden/commit/62e1da442fceae62115d1ac92450fc2751bea825))

## [1.2.0](https://github.com/bissquit/incident-garden/compare/v1.1.0...v1.2.0) (2026-01-28)


### Features

* implement cors middleware ([#11](https://github.com/bissquit/incident-garden/issues/11)) ([b1c13d7](https://github.com/bissquit/incident-garden/commit/b1c13d7998eb3f0dc374d0e0248db68f967c0ea6))

## [1.1.0](https://github.com/bissquit/incident-garden/compare/v1.0.0...v1.1.0) (2026-01-23)


### Features

* implement openapi spec ([#2](https://github.com/bissquit/incident-garden/issues/2)) ([0b05da0](https://github.com/bissquit/incident-garden/commit/0b05da024ad60025d4703b9125300ff34f65984b))
* initial working state with basic features ([#1](https://github.com/bissquit/incident-garden/issues/1)) ([dd6b5ee](https://github.com/bissquit/incident-garden/commit/dd6b5eed5a4bea57273a7c78a86cc077d1756de7))

## 1.0.0 (2026-01-22)


### Features

* implement openapi spec ([#2](https://github.com/bissquit/statuspage/issues/2)) ([0b05da0](https://github.com/bissquit/statuspage/commit/0b05da024ad60025d4703b9125300ff34f65984b))
* initial working state with basic features ([#1](https://github.com/bissquit/statuspage/issues/1)) ([dd6b5ee](https://github.com/bissquit/statuspage/commit/dd6b5eed5a4bea57273a7c78a86cc077d1756de7))

## [Unreleased]

### Added
- Initial release
- REST API for services, groups, events, templates
- JWT authentication with RBAC (user, operator, admin)
- Notification channels and subscriptions
- OpenAPI 3.0 specification
- Docker support with multi-stage builds
- Integration tests with testcontainers
- Automated releases with Release Please and GoReleaser
