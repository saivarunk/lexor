# ADR 001 — JSON files own the problem library

Status: accepted.

Use versioned JSON documents as the only authoritative problem definitions. Recursively validate and index them in memory; refresh after controlled writes or explicit rescan. SQLite contains progress, not a duplicate catalog.

This makes libraries inspectable, portable, versionable and usable in Git. It costs a scan and requires revision-aware atomic writes. Filesystem paths and logical topic paths remain distinct. Watching and cross-process transactions are deferred.
