use std::env;
use std::fs;
use std::path::PathBuf;

fn ensure_placeholder_resource_dir(resource_subdir: &str) {
    let manifest_dir = PathBuf::from(
        env::var("CARGO_MANIFEST_DIR").expect("CARGO_MANIFEST_DIR is not set"),
    );
    let resource_dir = manifest_dir.join("resources").join(resource_subdir);
    let placeholder_path = resource_dir.join("_discobot_placeholder.txt");

    fs::create_dir_all(&resource_dir).expect("Failed to create Tauri resource directory");

    let has_real_files = fs::read_dir(&resource_dir)
        .expect("Failed to read Tauri resource directory")
        .filter_map(Result::ok)
        .any(|entry| entry.file_name() != "_discobot_placeholder.txt");

    if has_real_files {
        let _ = fs::remove_file(&placeholder_path);
        return;
    }

    fs::write(
        &placeholder_path,
        "Generated during local builds so Tauri resource globs match before guest assets are extracted.\n",
    )
    .expect("Failed to write Tauri resource placeholder");
}

fn main() {
    match env::var("CARGO_CFG_TARGET_OS").as_deref() {
        Ok("macos") => ensure_placeholder_resource_dir("vz"),
        Ok("windows") => ensure_placeholder_resource_dir("wsl"),
        _ => {}
    }

    tauri_build::build()
}
