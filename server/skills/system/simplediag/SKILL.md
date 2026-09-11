---
name: simplediag
description: Create or repair network diagrams using SimpleDiag and nwdiag syntax. Use when the user asks for a network diagram, network topology, nwdiag, or SimpleDiag, including subnets, multi-homed hosts, firewalls, switches, and network groups.
metadata:
  title: SimpleDiag Network Diagrams (nwdiag)
  supported_runtimes:
    - native_sdk
    - codex
---

# SimpleDiag network diagrams

SimpleDiag is Helpin's renderer for **nwdiag** source. Use it to show hosts attached to network segments. Use Mermaid for workflows, request sequences, state machines, or data relationships instead, unless the user explicitly chooses a format.

## Output in Helpin

- Return a fenced `nwdiag` block in Markdown, including when writing document content. `simplediag` is the library name, not a supported fence language.
- Keep the source editable; no JavaScript imports, renderer installation, raw SVG, or image upload is needed for an inline diagram.
- When a document tool accepts TipTap JSON, use a `codeBlock` with `attrs.language` set to `nwdiag` and a text node containing the source. Use the tool's declared schema for the surrounding payload.
- Ground hosts, network membership, and addresses in the user's material. Identify illustrative topology as an example; do not invent actual infrastructure or imply that a connection proves firewall access.

## Syntax that matters

- Start with `nwdiag { ... }`. Declare each segment with `network identifier { ... }`.
- Use stable identifiers such as `web01`; use `description = "Web server"` for display labels. Quote addresses and labels and end statements with semicolons.
- A network's `address` labels its subnet. A node's `address` inside that network labels its attachment. Reuse the **same node identifier** in different networks for a multi-homed host; a new identifier creates a different host.
- A `group` visually groups named nodes; it does not connect them to a network.
- A peer link uses `a -- b`. SimpleDiag also supports attributes such as `[label = "TCP", style = dashed]`. For an explicit path through multiple hosts, use `route a -> b -> c`.
- Supported network shapes include `router`, `switch`, `firewall`, `server`, `client`, and `loadbalancer`; `database` and `cloud` are also useful. These network shapes are SimpleDiag extensions, not vendor icon names such as `cisco.router`.
- Helpin supplies light/dark colors. Leave fills and text colors unset by default. Favor short labels and split dense topologies by network boundary.

## Minimal example

Illustrative private network:

```nwdiag
nwdiag {
  network app_net {
    address = "10.20.0.0/24";
    api [description = "API", address = "10.20.0.10", shape = server];
    db [description = "Database", address = "10.20.0.20", shape = database];
  }
}
```

Read [references/examples.md](references/examples.md) for a firewall joining two networks, redundant hosts with groups, and aligned network rows with explicit routes.

## Verify and repair

Check balanced braces, quoted values, node identity, and network membership before returning the source. If a preview reports errors, correct the indicated line and retry. When a local JavaScript environment with SimpleDiag is already available, `renderFromSource(source, { errorMode: "null" })` returns `svg` and `diagnostics`: require a nonempty SVG and no diagnostics with severity `error`; investigate warnings before claiming success. Do not claim rendered validation when only inspecting source.

The examples target Helpin's SimpleDiag 0.2.4 integration. Consult the upstream [README](https://github.com/amadrizwan/simplediag) and [superset reference](https://github.com/amadrizwan/simplediag/blob/main/SUPERSET.md) for syntax beyond these examples. Features listed there as future ideas are not available syntax.
