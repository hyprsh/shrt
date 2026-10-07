# 4. Random codes, with optional chosen names

Date: 2026-10-07
Status: accepted

## Context

A short code becomes part of a URL that is shared and can't be changed
afterwards. A counter (`/x/1`, `/x/2`) is shortest, but anyone could walk
through every link. Andy also wants readable links now and then, such as
`hypr.sh/x/ferien`.

## Decision

- By default shrt generates the code: 6 random characters from
  `a-z A-Z 0-9` (62^6, about 57 billion), drawn from a cryptographic source.
- navi may instead pass a chosen name, limited to `a-z 0-9 -`. A name that
  is already taken is an error; shrt never overwrites a link.

## Consequences

- Links can't be enumerated or guessed.
- Generated codes are case-sensitive; chosen names are lowercase only.
- A generated code that collides is drawn again.
