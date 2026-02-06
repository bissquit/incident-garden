# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
