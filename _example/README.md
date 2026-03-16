# go-launcher examples

## Basic lifecycle

A minimal launcher + child pair that demonstrates process supervision, heartbeat, and clean shutdown.

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

### Try crash-loop detection

Edit `_example/child/main.go` and remove or comment out the `child.RequestShutdown()` call, then rebuild and rerun. The launcher will treat each exit as a crash, apply backoff delays, and exit with code 1 after reaching the crash threshold (3).

### Try Ctrl+C

While the child is running, press Ctrl+C. The launcher will send SIGTERM to the child and wait for it to exit gracefully.

## Auto-update demo

Demonstrates the full update lifecycle: v1 discovers an update, the launcher downloads v2 with checksum verification, rotates versions, and starts v2.

```bash
./_example/update-demo/run.sh
```

You should see:

```
=== go-launcher update demo ===

Building v1...
Building v2...
v2 checksum: sha256:72fbb8a...

--- launcher output ---

[child v1] starting up...
[child v1] heartbeat sent
[child v1] working...
[child v1] update available! requesting update to v2...
[child v1] exiting
INFO child requested update version=2.0.0
INFO updating child binary version=2.0.0 url=http://127.0.0.1:9384/demo-child
INFO checksum verified algo=sha256
[child v2] starting up — update successful!
[child v2] heartbeat sent
[child v2] working... (1/3)
[child v2] working... (2/3)
[child v2] working... (3/3)
[child v2] done, shutting down cleanly
[launcher] exiting with code 0
```

What happens under the hood:

1. The script builds two child binaries (v1 and v2), installs v1 as the current version, and serves v2 over a local HTTP server
2. The launcher starts v1
3. v1 signals healthy (heartbeat), then calls `child.RequestUpdate()` with v2's download URL and SHA-256 checksum
4. The launcher downloads v2 to `versions/staging/`, verifies the checksum, rotates `current/` → `previous/` and `staging/` → `current/`
5. The launcher starts v2, which runs normally and shuts down cleanly

The update flow exercises the same code paths as production — the only difference is the transport (local HTTP vs GitHub Releases).
