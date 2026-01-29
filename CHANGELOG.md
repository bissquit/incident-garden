# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
