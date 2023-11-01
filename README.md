# indieauth

> An IndieAuth toolkit in Go.

[![Go Report Card](https://goreportcard.com/badge/github.com/hacdias/indieauth?style=flat-square)](https://goreportcard.com/report/github.com/hacdias/indieauth)
[![Documentation](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://pkg.go.dev/github.com/hacdias/indieauth)

This repository contains a set of tools to help you implement [IndieAuth](https://indieauth.spec.indieweb.org/), both server and client, in Go. The repository documentation can be found [here](https://pkg.go.dev/github.com/hacdias/indieauth).

## Example

I am planning on writing an example server, and an example client. In the meanwhile, you can refer to the following places and the [documentation](https://pkg.go.dev/github.com/hacdias/indieauth):

- As client:
  - Discover endpoints and generate redirect URL: https://github.com/hacdias/eagle/blob/f59b3b6fc290e30b8c999edac4b3feadc6c3d7b6/server/login.go#L134-L167
  - Validate callback data: https://github.com/hacdias/eagle/blob/f59b3b6fc290e30b8c999edac4b3feadc6c3d7b6/server/login.go#L169-L192
- As server (a JWT token is used to store the information on the authorization code):
  - Parsing authorization request: https://github.com/hacdias/eagle/blob/f59b3b6fc290e30b8c999edac4b3feadc6c3d7b6/server/auth.go#L29-L46
  - Validate the code challenge at the authorization endpoint: https://github.com/hacdias/eagle/blob/58bce1373527b997edcac2387f0ee38328e13f10/server/auth.go#L154-L165

## Contribute

Just open an issue or a pull request.

## License

[MIT License](LICENSE) © [Henrique Dias](https://hacdias.com)
