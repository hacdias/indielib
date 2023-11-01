# IndieAuth Client Demo

This is a client demo using the [`indieauth`](../../) package. In this demo, an HTTP server is started where a user can use IndieAuth to log in. After navigating to the page, type your domain, and login. How to's:

- How to run: `go run .`
- How to get help: `go run . --help` and read the [commented code](main.go)

The example works best if not run in localhost. You can try Tailscale Funnel, ngrok, or other similar services to temporarily expose the service. This URL has to be passed via the `--client` flag.

## Using Tailscale Funnel

If you use Tailscale, you can easily use their Funnel functionality to temporarily expose the server in a publicly reachable address. Start the funnel as follows:

```console
$ tailscale funnel 3000
Available on the internet:

https://your-machine.and.your.ts.net/
|-- proxy http://127.0.0.1:3000

Press Ctrl+C to exit.
```

And then the demo:

```console
$ go run .  --port 3000 --client "https://your-machine.and.your.ts.net/"
2023/11/02 13:03:52 Listening on http://localhost:3000
2023/11/02 13:03:52 Listening on https://your-machine.and.your.ts.net/
```

Navigate to `https://your-machine.and.your.ts.net/`.
