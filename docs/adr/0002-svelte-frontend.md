# ADR-0002 — Svelte for the frontend

- Status: Accepted
- Date: 2026-06

## Context

Conductor's UI is a dense control surface (tables, live stats), not a content
site. We want minimal webview runtime overhead and low ceremony.

## Decision

Use Svelte + TypeScript + Vite.

## Consequences

Smaller bundle and simpler state than React. Svelte's reactivity fits the live
transfer-stat stream well. State lives in defined stores (§7.11.10), with
runtime (live stats) and view state kept distinct.
