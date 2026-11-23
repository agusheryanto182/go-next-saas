# 0016. PWA: cache the app shell, never the API

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
Carried over from employee-portal: the app should be installable and open offline, but cached private data on a shared or lost device is a risk, and stale data is confusing.

## Decision
Use Serwist in the Next.js app: precache the shell (scripts, styles, icons, offline page), never cache `/api`, show an offline banner and fallback, and prompt the user to reload when a new version is available. The service worker and manifest are served without long-lived caching.

## Alternatives considered
- **Cache API responses for offline reading:** private data at rest on the device, plus invalidation rules.
- **No PWA:** loses installability on phones.

## Consequences
- Installable and starts offline; data needs a connection.
- Serwist must keep pace with Next.js releases.
