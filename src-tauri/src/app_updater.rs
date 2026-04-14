use std::time::Duration;

use serde::Serialize;
use tauri::{ipc::Channel, Manager, Resource, ResourceId, Runtime, Webview};
use tauri_plugin_updater::{Update, UpdaterExt};
use url::Url;

#[derive(Debug, Clone, Serialize)]
#[serde(tag = "event", content = "data")]
pub(crate) enum DownloadEvent {
    #[serde(rename_all = "camelCase")]
    Started {
        content_length: Option<u64>,
    },
    #[serde(rename_all = "camelCase")]
    Progress {
        chunk_length: usize,
    },
    Finished,
}

#[derive(Serialize, Default)]
#[serde(rename_all = "camelCase")]
pub(crate) struct UpdateMetadata {
    rid: ResourceId,
    current_version: String,
    version: String,
    date: Option<String>,
    body: Option<String>,
    raw_json: serde_json::Value,
}

struct DownloadedBytes(Vec<u8>);

impl Resource for DownloadedBytes {}

fn format_update_metadata<R: Runtime>(
    webview: &Webview<R>,
    update: Update,
) -> Result<UpdateMetadata, String> {
    let formatted_date = if let Some(date) = update.date {
        Some(
            date.format(&time::format_description::well_known::Rfc3339)
                .map_err(|_| "Failed to format update publication date".to_string())?,
        )
    } else {
        None
    };

    Ok(UpdateMetadata {
        current_version: update.current_version.clone(),
        version: update.version.clone(),
        date: formatted_date,
        body: update.body.clone(),
        raw_json: update.raw_json.clone(),
        rid: webview.resources_table().add(update),
    })
}

#[tauri::command]
pub(crate) async fn check_for_app_update<R: Runtime>(
    webview: Webview<R>,
    endpoint: Option<String>,
) -> Result<Option<UpdateMetadata>, String> {
    let mut builder = webview.updater_builder().timeout(Duration::from_secs(30));

    if let Some(endpoint) = endpoint {
        builder = builder
            .endpoints(vec![Url::parse(&endpoint)
                .map_err(|err| format!("Invalid update endpoint URL: {err}"))?])
            .map_err(|err| format!("Failed to configure updater endpoint: {err}"))?;
    }

    let updater = builder
        .build()
        .map_err(|err| format!("Failed to build updater: {err}"))?;
    let update = updater
        .check()
        .await
        .map_err(|err| format!("Failed to check for updates: {err}"))?;

    update
        .map(|update| format_update_metadata(&webview, update))
        .transpose()
}

#[tauri::command]
pub(crate) async fn download_app_update<R: Runtime>(
    webview: Webview<R>,
    rid: ResourceId,
    on_event: Channel<DownloadEvent>,
) -> Result<ResourceId, String> {
    let update = webview
        .resources_table()
        .get::<Update>(rid)
        .map_err(|err| format!("Failed to load update resource: {err}"))?;

    let mut first_chunk = true;
    let bytes = update
        .download(
            |chunk_length, content_length| {
                if first_chunk {
                    first_chunk = false;
                    let _ = on_event.send(DownloadEvent::Started { content_length });
                }
                let _ = on_event.send(DownloadEvent::Progress { chunk_length });
            },
            || {
                let _ = on_event.send(DownloadEvent::Finished);
            },
        )
        .await
        .map_err(|err| format!("Failed to download update: {err}"))?;

    Ok(webview.resources_table().add(DownloadedBytes(bytes)))
}

#[tauri::command]
pub(crate) async fn install_app_update<R: Runtime>(
    webview: Webview<R>,
    update_rid: ResourceId,
    bytes_rid: ResourceId,
) -> Result<(), String> {
    let update = webview
        .resources_table()
        .get::<Update>(update_rid)
        .map_err(|err| format!("Failed to load update resource: {err}"))?;
    let bytes = webview
        .resources_table()
        .get::<DownloadedBytes>(bytes_rid)
        .map_err(|err| format!("Failed to load downloaded update: {err}"))?;

    update
        .install(&bytes.0)
        .map_err(|err| format!("Failed to install update: {err}"))?;

    let _ = webview.resources_table().close(bytes_rid);
    let _ = webview.resources_table().close(update_rid);
    Ok(())
}

#[tauri::command]
pub(crate) fn close_app_update<R: Runtime>(
    webview: Webview<R>,
    update_rid: Option<ResourceId>,
    bytes_rid: Option<ResourceId>,
) {
    if let Some(bytes_rid) = bytes_rid {
        let _ = webview.resources_table().close(bytes_rid);
    }
    if let Some(update_rid) = update_rid {
        let _ = webview.resources_table().close(update_rid);
    }
}
