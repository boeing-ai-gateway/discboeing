# macOS Entitlements

Discobot's desktop app is packaged with Electron. The macOS entitlement file used
by electron-builder lives at `electron/assets/entitlements.plist`.

The app needs permissions for JIT compilation, networking, file access,
automation, and Apple's Virtualization framework. The virtualization entitlement
allows the Electron wrapper and its bundled `discobot-server` sidecar to start
and manage the local VZ guest runtime.

## Build and verify

Build the packaged app with:

```bash
pnpm dist:app --mac arm64
```

Verify the signed app's entitlements with:

```bash
codesign -d --entitlements - dist/mac-arm64/Discobot.app
```

The output should include:

```xml
<key>com.apple.security.virtualization</key>
<true/>
```

## Related files

```text
electron/
├── assets/
│   ├── entitlements.plist
│   └── icons/
└── resources/
    └── vz/
```

`package.json` references the entitlement file from the Electron builder `mac`
configuration.
