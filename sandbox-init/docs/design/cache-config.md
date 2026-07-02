# Cache Configuration

Workspaces can request additional persistent cache directories with
`.discboeing/cache.json`.

## File Format

```json
{
  "additionalPaths": ["/home/discboeing/.cache/custom-tool"]
}
```

Only absolute paths under `/home/discboeing` are accepted for custom entries.
Invalid paths are ignored during startup.

## Startup Behavior

`sandbox-init/discboeing-sandbox-init.sh` mounts caches after the OverlayFS home is
mounted:

1. It emits the built-in well-known cache path list.
2. It reads `.discboeing/cache.json` from `/home/discboeing/workspace` when present.
3. For each valid path, it creates a matching source under `/.data/cache`.
4. It creates the target path in the overlay home.
5. It bind-mounts the cache source over the target.

Set `CACHE_ENABLED=false` to skip cache bind mounts.
