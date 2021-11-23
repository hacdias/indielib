# IndieAuth Toolkit for Go

[![Go Report Card](https://goreportcard.com/badge/github.com/hacdias/indieauth?style=flat-square)](https://goreportcard.com/report/github.com/hacdias/indieauth)
[![Documentation](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://pkg.go.dev/github.com/hacdias/indieauth)

This repository contains a set of tools to help you implement IndieAuth, both server and client, in Go. The documentation can be found [here](https://pkg.go.dev/github.com/hacdias/indieauth). Please note that there may be bugs. Feel free to use it and send PRs with improvements.

## Usage

Some examples:

- As client:
  - Discover endpoints and generate redirect URL: https://github.com/hacdias/eagle/blob/f59b3b6fc290e30b8c999edac4b3feadc6c3d7b6/server/login.go#L134-L167
  - Validate callback data: https://github.com/hacdias/eagle/blob/f59b3b6fc290e30b8c999edac4b3feadc6c3d7b6/server/login.go#L169-L192
- As server:
  - Parsing authorization request: https://github.com/hacdias/eagle/blob/f59b3b6fc290e30b8c999edac4b3feadc6c3d7b6/server/auth.go#L29-L46
  - Validate the code challenge at the authorization endpoint: https://github.com/hacdias/eagle/blob/f59b3b6fc290e30b8c999edac4b3feadc6c3d7b6/server/auth.go#L262-L268

## License

MIT © Henrique Dias
