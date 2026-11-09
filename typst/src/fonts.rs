//! Where fonts come from: the system's, then those bundled with Typst. Among
//! faces that match a request equally Typst takes the first, so an installed
//! copy of a bundled face wins.

use std::panic::{AssertUnwindSafe, catch_unwind};
use std::path::PathBuf;
use std::sync::OnceLock;

use typst::text::FontInfo;
use typst_kit::fonts::{FontPath, FontStore};

/// A face: its file, its index in the file, and what the font book knows of it.
type Face = (PathBuf, u32, FontInfo);

/// The system's faces, scanned once for the process: a scan reads the tables
/// of every font file, and every open document has a compiler of its own.
/// None when the scan panics: fontdb expects a file it listed to still be
/// there when it reads it.
fn system() -> &'static [Face] {
    static FACES: OnceLock<Vec<Face>> = OnceLock::new();
    FACES.get_or_init(|| {
        let found = catch_unwind(AssertUnwindSafe(|| typst_kit::fonts::system().collect::<Vec<_>>())).unwrap_or_default();
        found.into_iter().map(|(at, info)| (at.path, at.index, info)).collect()
    })
}

/// Scans the system's fonts on a thread of its own, so that the first
/// compiler waits for what is left of the scan and not for all of it.
pub fn warm() {
    std::thread::spawn(|| {
        system();
    });
}

/// A compiler's fonts, fixed for the compiler they are made for.
pub fn store() -> FontStore {
    let mut fonts = FontStore::new();
    fonts.extend(system().iter().cloned().map(|(path, index, info)| (FontPath { path, index }, info)));
    fonts.extend(typst_kit::fonts::embedded());
    fonts
}
