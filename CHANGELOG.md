# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

### Changed

### Deprecated

### Removed

### Fixed

### Security

## [0.4.4]

### Changed

- Upgraded dependencies to latest versions.

## [0.4.3]

### Changed

- Micropub: the `Commands` field will also be available for update requests.

## [0.4.2]

### Changed

- Upgraded dependencies to latest versions.

## [0.4.1]

### Changed

- Upgraded dependencies to latest versions.

## [0.4.0]

### Changed

- Upgraded dependencies to latest versions.
- Upgrades build to use Go 1.23.

## [0.3.1]

### Changed

- Upgraded dependencies to newest versions.

## [0.3.0]

### Changed

- IndieAuth: added `context.Context` as argument to all functions that perform HTTP requests.
- IndieAuth: added `DiscoverApplicationMetadata` to the `Server`, which implements the [Application Information Discovery](https://indieauth.spec.indieweb.org/#application-information).

## [0.2.3]

### Changed

- Micropub: set `text/plain` content-type on Micropub create responses, allowing for compatibility with clients that expect it, such as [Sparkles](https://sparkles.sploot.com/).

## [0.2.2]

### Fixed

- Micropub: fixes panic when `WithGetVisibility` was not provided.

## [0.2.1]

### Added

- Micropub: new `WithMaxMemory` added for the media handler.

### Fixed

- Micropub: the `WithMaxMediaSize` option for the media handler is now properly honoured. Thanks [@jlelse](https://jlelse.blog/) for reporting this.

## [0.2.0]

### Added

- Micropub: add support for [Visibility](https://indieweb.org/Micropub-extensions#Visibility) in the configuration.
- Micropub: add support for [Post Listing](https://indieweb.org/Micropub-extensions#Query_for_Post_List), which involves adding a new function to the Implementation interface.

### Changed

- Micropub: the keys of Request.Commands no longer start with `mp-`.

## [0.1.0]

### Added

- Migrated `go.hacdias.com/indieauth` into `go.hacdias.com/indielib/indieauth`.
- Added Micropub related tooling.
