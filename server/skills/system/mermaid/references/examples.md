# Mermaid example patterns

Adapt these illustrative examples to verified facts. Preserve the `mermaid` fence when embedding in Helpin Markdown.

## API sequence with success and failure

Participant aliases keep identifiers stable. Each branch returns a result to the caller; use `loop` or `opt` only if the actual behavior requires it.

```mermaid
sequenceDiagram
    actor Reader
    participant API as Document API
    participant Store as Document store
    Reader->>API: Request an article
    API->>Store: Read article and visibility
    Store-->>API: Article metadata
    alt Reader has access
        API-->>Reader: Article content
    else Access denied
        API-->>Reader: Permission error
    end
```

## Entity relationships

Here a workspace contains zero or more documents, while each document belongs to exactly one workspace. A document has zero or more revisions. Use these cardinalities only when the real schema supports them.

```mermaid
erDiagram
    WORKSPACE ||--o{ DOCUMENT : contains
    DOCUMENT ||--o{ REVISION : records
    WORKSPACE {
        uuid id PK
        string name
    }
    DOCUMENT {
        uuid id PK
        uuid workspace_id FK
        string title
    }
    REVISION {
        uuid id PK
        uuid document_id FK
        int version
    }
```

## Class structure

Composition (`*--`) means ownership of the part's lifetime; dependency (`..>`) means use without ownership. Use inheritance (`<|--`) only for a real subtype relationship.

```mermaid
classDiagram
    Document "1" *-- "0..*" Revision : owns
    Publisher ..> Document : reads
    class Document {
        +string title
        +latestRevision()
    }
    class Revision {
        +int version
        +string content
    }
    class Publisher {
        +publish()
    }
```

## Lifecycle states

Transitions are events, not a list of tasks. This example permits a review to return to draft before publication.

```mermaid
stateDiagram-v2
    [*] --> Draft
    Draft --> InReview: Submit
    InReview --> Draft: Request changes
    InReview --> Published: Approve
    Published --> Archived: Retire
    Archived --> [*]
```

## Timeline dependencies

These dates and durations are illustrative. `review` begins after `draft` finishes, and the release milestone follows `review`.

```mermaid
gantt
    title Documentation release example
    dateFormat YYYY-MM-DD
    section Documentation
    Draft guide :draft, 2026-10-01, 3d
    Technical review :review, after draft, 2d
    Release :milestone, release, after review, 0d
```
