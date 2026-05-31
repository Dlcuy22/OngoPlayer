# NOTICE: Gio UI Deprecation

**Status:** Deprecated (Maintenance Mode Only)

The Gio-based native UI (`cmd/gui` and `internal/ui/gio`) is officially deprecated. All future feature development, UI iterations, and enhancements will be focused entirely on the **WebUI** (`cmd/webui` using Wails and Svelte).

## Why the shift?
While the Gio UI provided a very lightweight footprint and single-binary deployment, its immediate-mode rendering architecture presented severe scaling issues for the future of OngoPlayer:

1. **GPU/CPU Overhead with Animations:** Immediate-mode UI requires the *entire* layout tree to be rebuilt and re-submitted to the GPU 30+ times per second during continuous animations (like synced lyrics or future visualizers). This caused unacceptable base load, especially on Windows through the ANGLE translation layer.
2. **Feature Scalability:** Adding complex interactive panels (10-band EQs, DSP controls, fluid animations) multiplies this overhead.
3. **Styling & Ecosystem:** The web ecosystem provides infinitely more flexible styling (CSS), a massive library of ready-to-use accessible components, and independent GPU-accelerated compositing (e.g., HTML5 `<canvas>` for visualizers) that doesn't trigger full UI repaints.

## What this means
* **No new features** will be added to the Gio UI.
* The code remains in the repository as a fallback/legacy option for extremely low-resource environments that do not want the WebView2/WebKit overhead.
* If you are looking to contribute UI features, please target the `cmd/webui` application.

*- The OngoPlayer Development Team*
