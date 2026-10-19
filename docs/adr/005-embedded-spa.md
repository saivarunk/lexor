# ADR 005 — Build and embed the React SPA

Status: accepted.

Keep frontend assets inside the web Go package to respect embed package boundaries. Bun/Vite builds hashed assets, then Go embeds them. API routing has an explicit JSON fallback and cannot accidentally return index.html. Source-only Go development has an explanatory unbuilt-assets page.

The production binary needs no Bun/Node to serve the UI, and all editor workers/assets are local. Building from source requires both toolchains and the ordered Makefile. Monaco is a substantial lazy chunk; initial library/dashboard bundles stay separate. A service worker is unnecessary for a locally running offline server.
