# 1. The HTTP API is the only interface

Date: 2026-10-07
Status: accepted

## Context

Andy shortens URLs by asking navi, the household AI assistant. Nobody else
creates links. A web form or CLI would be a second way in, with its own auth,
for a use that goes through navi anyway.

## Decision

shrt has one interface for managing links: an HTTP API (create, list,
delete), authenticated with a bearer token that navi holds. There is no web
UI and no CLI.

## Consequences

- One small codebase and one auth path.
- Andy can't create links without navi (or curl). If he misses that, a web
  form can be added later on top of the same API.
