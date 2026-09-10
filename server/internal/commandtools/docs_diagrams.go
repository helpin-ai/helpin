package commandtools

// Keep diagram authoring guidance on the live Docs tools so all agents that can
// write documents receive it without loading an optional skill.
const docsDiagramGuidance = " Helpin Docs renders editable nwdiag network diagrams using simplediag. Use nwdiag for network topology (networks, hosts, addresses, and hosts connected to multiple networks); keep Mermaid for flowcharts and sequence diagrams. Use the user's supplied or verified topology; label illustrative examples and do not invent infrastructure facts. In nwdiag source, use nwdiag { network name { ... } }, quote address and label values, and end attribute assignments and node declarations with semicolons. Reuse a host identifier in multiple networks to connect it to each. Keep the diagram as editable source, not an SVG or image."

const nwdiagSourceExample = `nwdiag {
  network dmz {
    address = "10.0.0.0/24";
    web01 [address = ".10"];
    web02 [address = ".11"];
  }
}`

const docsDiagramMarkdownGuidance = " For a network diagram, use a fenced code block with language nwdiag. Example (illustrative addresses):\n```nwdiag\n" + nwdiagSourceExample + "\n```"

const docsDiagramJSONGuidance = ` For a network diagram, use a codeBlock node with attrs.language set to "nwdiag" and raw nwdiag source in its text child, not a node whose type is "nwdiag". Example: {"type":"codeBlock","attrs":{"language":"nwdiag"},"content":[{"type":"text","text":"nwdiag { network dmz { web01; web02; } }"}]}.`
