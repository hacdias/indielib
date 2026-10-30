# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

### Changed

- IndieAuth: `NewClient` and `NewServer` now default to an HTTP client with a 10 second timeout instead of `http.DefaultClient`, which has none.
- IndieAuth: the default clients of `NewClient` and `NewServer` no longer send requests through `http.DefaultTransport`, so mocks that replace it, such as httpmock or gock, don't intercept them. Pass `http.DefaultClient` to keep that.
- IndieAuth: `Metadata.ServiceDocumentation` is now a `string`, matching the [specification](https://indieauth.spec.indieweb.org/#indieauth-server-metadata). Before, metadata that included it failed to decode and discovery fell back to the legacy link relations, or failed.

### Deprecated

### Removed

### Fixed

- IndieAuth: endpoint discovery now matches `rel` values separated by any whitespace, such as a newline, not only by spaces.
- IndieAuth: `CanonicalizeURL` now recognizes the scheme in any case, such as `Https://` from mobile keyboards, and lowercases the host. Before, `Https://example.com` became `https://Https://example.com`.
- IndieAuth: `Server.ParseAuthorization` no longer returns empty scopes when `scope` has repeated or surrounding spaces.
- IndieAuth: `Client.Authenticate` no longer panics when the server metadata is `null`. Metadata without an `authorization_endpoint` is treated as not found, and discovery falls back to the legacy link relations. `Client.FetchProfile` no longer returns a nil profile for a `null` response.
- IndieAuth: `IsValidProfileURL` and `IsValidClientIdentifier` now reject only `.` and `..` path segments, as the specification says, instead of any dot in the path. URLs like `https://example.com/jane.doe` are now valid.
- Microformats: `DiscoverType` now collapses whitespace in `name` and `content`, and reads plain-string `content`. Both cases were wrongly reported as articles.
- Micropub: `?q=config` no longer panics when `WithGetPostStatuses` is not set.
- Micropub: `ParseRequest` no longer removes `h` and `access_token` from `r.Form`, so `HasScope` can read a token sent in the form body.
- Micropub: `WithMaxMemory` now sets the memory limit instead of overriding the maximum media size.
- Micropub: `ParseRequest` rejects `action=create` without a type, which previously reached `Create` with nil `Properties`. JSON requests with both `type` and `action` are now rejected with `ErrNoActionCreate`, like form-encoded ones.

### Security

- IndieAuth: responses from remote URLs (metadata, profiles, client pages) are read up to 1 MiB. Before, a remote server could make the client or server use unbounded memory.
- Micropub: JSON request bodies are limited to 10 MiB, like form-encoded ones, and larger bodies return `ErrTooLarge`.
- IndieAuth: `Client.FetchProfile` and `Client.GetToken` now verify the returned `me`, as the [specification](https://indieauth.spec.indieweb.org/#authorization-server-confirmation) requires. It must be a valid profile URL and, if it differs from the profile URL used to authenticate, it must declare the same authorization endpoint, or `ErrInvalidMe` is returned. Before, a malicious authorization server could claim any profile URL.
- IndieAuth: the default clients of `NewClient` and `NewServer` refuse to connect to loopback addresses and return `ErrLoopback`. The server no longer fetches loopback client identifiers during redirect URI discovery or `DiscoverApplicationMetadata`, as the [specification](https://indieauth.spec.indieweb.org/#client-information-discovery) requires. The client no longer follows links on a user's profile page to loopback addresses. This covers redirects, domains that resolve to loopback, and proxies on loopback. A client passed to `NewClient` or `NewServer` is used as is, without this check, for example to test locally.
- IndieAuth: `Server.ParseAuthorization` now requires the `redirect_uri` to also match the scheme of the `client_id` before skipping redirect URI discovery. Previously, `http://app.example/cb` was accepted for `https://app.example/`.
- IndieAuth: `IsValidProfileURL` and `IsValidClientIdentifier` no longer accept IP addresses written with a port, as IPv6, or with an IPv6 zone, such as `https://10.0.0.1:8080/`, `https://[2001:db8::1]/` or `https://[fe80::1%25en0]/`.

## [0.6.0]

### Added

- IndieAuth: `Server.ParseAuthorization` now accepts a `redirect_uri` on a different host than the `client_id` if the client publishes it, via `redirect_uris` in the [client metadata](https://indieauth.spec.indieweb.org/#client-metadata) or via `redirect_uri` [link relations](https://indieauth.spec.indieweb.org/#redirect-url).

### Changed

- Upgraded dependencies to latest versions.
- Upgrades build to use minimum Go 1.27.

## [0.5.0]

### Added

- Micropub: added `WithGetPostStatuses`, which allows you to configure the [Post Statuses](https://indieweb.org/Micropub-extensions#Post_Status).

### Changed

- Upgraded dependencies to latest versions.
- Upgrades build to use minimum Go 1.24.

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
