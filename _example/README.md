# go-launcher example

A minimal launcher + child pair that demonstrates the full supervisor lifecycle.

## Run it

```bash
# Build the child binary into the versions directory
go build -o /tmp/go-launcher-example/versions/current/example-child ./_example/child/

# Run the launcher
go run ./_example/launcher/
```

You should see:

```
[launcher] starting supervisor loop...
[child] starting up...
[child] heartbeat sent to launcher
[child] working... (1/5)
[child] working... (2/5)
[child] working... (3/5)
[child] working... (4/5)
[child] working... (5/5)
[child] requesting clean shutdown
[child] exiting
[launcher] exiting with code 0
```

## Try crash-loop detection

Edit `_example/child/main.go` and remove or comment out the `child.RequestShutdown()` call, then rebuild and rerun. The launcher will treat each exit as a crash, apply backoff delays, and exit with code 1 after reaching the crash threshold (3).

## Try Ctrl+C

While the child is running, press Ctrl+C. The launcher will send SIGTERM to the child and wait for it to exit gracefully.
