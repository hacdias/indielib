# Micropub and IndieAuth Server Demo

This is a server demo using the [`indiekit`](../../) package. In this demo, an HTTP server is started and is used as the user's identity.

- How to run: `go run .`
- How to get help: `go run . --help` and read the [commented code](main.go)

The example works best if not run in localhost. You can try Tailscale Funnel, ngrok, or other similar services to temporarily expose the service. This URL has to be passed via the `--client` flag.

Note that this demo does not implement all functionalities from IndieAuth/OAuth2.

You can try this demo with applications such as [Quill](https://quill.p3k.io/), or [Micropublish](https://micropublish.net/).

## Using Tailscale Funnel

If you use Tailscale, you can easily use their Funnel functionality to temporarily expose the server in a publicly reachable address. Start the funnel as follows:

```console
$ tailscale funnel 5050
Available on the internet:

https://your-machine.and.your.ts.net/
|-- proxy http://127.0.0.1:5050

Press Ctrl+C to exit.
```

And then the demo:

```console
$ go run . --port 5050 --profile "https://your-machine.and.your.ts.net/"
2023/11/02 13:03:52 Listening on http://localhost:5050
2023/11/02 13:03:52 Listening on https://your-machine.and.your.ts.net/
```

Now you can navigate to a website that supports logging in with IndieAuth, and use `https://your-machine.and.your.ts.net/` as your identification. You will be then redirected to this demo to authorize the request.
