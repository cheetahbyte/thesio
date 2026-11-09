# System fonts

**Status:** implemented

## Problem

A document could only use the four font families bundled with Typst (Libertinus Serif, New Computer Modern, New Computer Modern Math, DejaVu Sans Mono). Naming any other family in `#set text(font: ...)` gave Typst's unknown-font warning and the text fell back to a bundled font. Someone writing a thesis against a university template, which usually prescribes a font, could not produce the required output in Thesio.

## Solution

Documents can use every font installed on the machine. Nothing has to be configured: an installed family works in the preview and in the exported PDF.

## Behavior

1. A document names a family installed on the machine. The preview and the exported PDF use it, and no unknown-font warning appears.
2. A document names a family that is not installed. Typst's unknown-font warning shows in the problems list as before, and the text falls back as before.
3. The same face (family, style, weight and stretch) is both installed and bundled. The installed file is used. Precedence is per face: Typst first picks the closest style, weight and stretch across both sources and prefers a variable font over a static one, and only then does the order break a tie.
4. A font is installed while the app runs. It is available after the app restarts.
5. Bundled fonts stay available and stay the default family. Existing documents can still render differently than before, and differently per machine, in three cases: characters the bundled fonts lack (CJK, emoji, many symbols) now fall back to a system font; a font list that already named a system family before a bundled one now uses the system family; and text inside SVG images resolves against the same fonts.
6. Font-name completion in `#set text(font: ...)` offers every installed family.

## Decisions

- **Only system fonts, no fonts folder in the vault** (decided by the user). A `fonts/` folder at the vault root, with a settings page to open it, was built and then removed: the user's view is that people use fonts they have installed, so a second place for fonts is not worth having. The cost is that a document naming a non-standard font does not compile the same on a machine without that font.
- **System before bundled.** An installed copy of a face is used before Typst's bundled copy.
- **The system scan result is kept once per process; each compiler builds its own font store from it.** A compiler is created for every opened document and for every PDF export, and `typst-kit`'s font store can be neither cloned nor merged, so what is shared is the list of found faces (path, index in the file, metadata). Font bytes are therefore loaded per compiler, on first use of a face.
- **The system scan starts in the background when the app starts.** On the development Mac the scan of 1303 faces took about 0.9 s the first time and about 0.12 s on repeats, and a compiler is created on the UI thread when a document opens. Scanning at start, off the UI thread, keeps the first document from freezing the window; opening a document before the scan ends waits only for the rest of it.
- **A failed scan leaves the bundled fonts.** `typst-kit`'s scan can panic when a font file vanishes while it is read; the panic is caught and the system contributes no fonts for that run.
- **Adobe Fonts are included.** `typst_kit::fonts::system()` adds the fonts synced by Adobe's app on macOS and Windows with no way to opt out. It is also what the Typst CLI does.
- **Font discovery ships to Linux and Windows unverified** (accepted by the user). The release workflow builds both, and on Linux the scan goes through fontconfig.

## Interfaces and data

- **C ABI (`typst/include/thesio_typst.h`, `typst/src/ffi.rs`):** `thesio_fonts_warm` starts the system font scan and returns at once. Calling it is optional.
- **Go API (`internal/typst`):** `WarmFonts` wraps it; `main.go` calls it at start.
- **Vault format:** no change.
- **Rust dependencies:** `typst-kit` gains its `scan-fonts` feature. The crates it needs were already in `typst/Cargo.lock`; no new crate.

## Affected code

- `typst/Cargo.toml`: `scan-fonts` on `typst-kit`.
- `typst/src/fonts.rs`, new: the process-wide list of system faces, and the font store a compiler is made with.
- `typst/src/compiler.rs`: `Compiler::with_packages` takes its fonts from `fonts::store`.
- `typst/src/ffi.rs`, `typst/include/thesio_typst.h`, `internal/typst/compiler.go`, `main.go`: starting the scan.
- `docs/ARCHITECTURE.md`, `CHANGELOG.md`: the limit removed and the behavior described.

## Testing

Installed fonts differ per machine, so no test names one. The feature is covered by the existing Rust and Go suites passing with system fonts enabled. A one-off check on the development Mac compiled documents naming Helvetica Neue and Times New Roman without the warning, and a made-up family with it.

## Out of scope

- A fonts folder in the vault, a font picker, or any font UI.
- Noticing fonts installed while the app runs.
- A setting to turn system fonts off.

## Open questions

1. **Does a packaged macOS build read the system and user font folders without extra entitlements?** Not checked. To be verified with a bundled build before release.
2. **How much memory do per-compiler font bytes cost with several documents open?** Not measured.
