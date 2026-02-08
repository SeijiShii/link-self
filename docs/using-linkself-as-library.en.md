# Using LinkSelf as a Library

**日本語** ([using-linkself-as-library.md](using-linkself-as-library.md)) | **English** (this page)  
**Status:** To be fleshed out in Phase 2 (principles and usage examples only)  
**Summary:** How to use LinkSelf (Go core) as a library on each platform: principles, platform-specific integration, and examples.  
**See also:** [Phase 1 design](phase1-design.en.md), [Sample app plan](sample-chat-app-plan.en.md), [Roadmap](README.en.md#roadmap)

---

## Principles

- **Library on every platform** — Expose LinkSelf (Go core) so each platform can call it as a library. Stable public API, packaging, and usage will be documented in Phase 2.
- **Platform-appropriate** — Use the UI and integration style that fit each platform. UI framework is chosen per platform (e.g. Electron, Flutter, native).

---

## Platform-specific integration (examples)

| Platform | Go delivery (example) | UI options |
|----------|------------------------|------------|
| Windows  | DLL (CGO) + FFI / or subprocess (stdio/socket) | Electron, WinUI, WPF, Flutter, etc. |
| macOS    | .dylib + FFI / or subprocess | Electron, Swift/SwiftUI, Flutter, etc. |
| Linux    | .so + FFI / subprocess | Electron, GTK, Flutter, etc. |
| Android  | gomobile → AAR        | Kotlin/Compose, Flutter, etc. |
| iOS      | gomobile → Framework  | Swift/SwiftUI, Flutter, etc. |

---

## Usage examples (Windows)

### Windows + Electron

- **Subprocess:** Run Go as a `linkself` binary (or dedicated daemon); from Electron’s main process, communicate via stdio or TCP/socket (e.g. JSON-RPC). Simple to implement and keeps Go changes minimal.
- **Node FFI:** Build Go as a C shared library (DLL) and call it from Node with `node-ffi-napi` (or similar). Single process, but requires CGO build and ABI setup.

### Windows + Flutter

- Build Go as a C dynamic library (DLL) and call it via **dart:ffi**. Alternatively, run Go as a subprocess and communicate over stdio/socket. Phase 2 will add build steps and FFI binding examples.

---

## Planned work (Phase 2)

- Stabilize and document the public API.
- Packaging per platform (DLL / AAR / Framework / binary) and build instructions.
- Sample code or step-by-step guides for the usage examples above.
