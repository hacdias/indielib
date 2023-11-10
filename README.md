# indiekit

[![Go Report Card](https://goreportcard.com/badge/go.hacdias.com/indiekit?style=flat-square)](https://goreportcard.com/report/go.hacdias.com/indiekit)
[![Documentation](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://pkg.go.dev/go.hacdias.com/indiekit)
[![Codecov](https://img.shields.io/codecov/c/github/hacdias/indiekit?token=SSETVGG0UH&style=flat-square)](https://app.codecov.io/gh/hacdias/indiekit)

An [IndieWeb](https://indieweb.org/) toolkit in Go. This repository contains a set of tools to help you implement IndieWeb related protocols in Go: [IndieAuth](https://indieauth.spec.indieweb.org) client and server, [Micropub](https://micropub.spec.indieweb.org/) server, and [Microformats](https://microformats.org/wiki/microformats2) [post discovery](https://www.w3.org/TR/post-type-discovery/).

## Install

```
go get go.hacdias.com/indiekit@latest
```

## Usage

Check the [documentation](https://pkg.go.dev/go.hacdias.com/indiekit). This repository also contains two illustrative examples: a [server](examples/server/) and a [client](examples/client/).

## Other Packages

Below is a list of other IndieWeb-related Go packages that can help you implement whatever feature you are looking for to implement:

- [willnorris.com/go/microformats](https://willnorris.com/go/microformats/) - parsing Microformats.
- [willnorris.com/go/webmention](https://willnorris.com/go/webmention/) - library and CLI for sending Webmentions.

## Contributing

Feel free to open an issue or a pull request.

## License

[MIT License](LICENSE) © [Henrique Dias](https://hacdias.com)
