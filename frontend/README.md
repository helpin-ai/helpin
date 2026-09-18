# Helpin frontend

This is Helpin's React and TypeScript application. Use this guide to run or
validate the frontend; start with the [local development guide](../docs/development.md)
for API, database, and worker setup.

## Run locally

Use the Node version in [`.node-version`](../.node-version) and pnpm version in
[`package.json`](../package.json). Run these commands from the repository root:

```sh
pnpm install --frozen-lockfile
pnpm --filter @helpin-ai/widget-core build
VITE_API_URL=http://localhost:8080/api pnpm --dir frontend dev
```

Open `http://localhost:5173`. The dev script generates icons before starting Vite.
The shared widget build is required because Vite resolves its compiled output.

For a browser on another machine, replace `localhost` in `VITE_API_URL` with the
API host reachable from that browser and configure backend CORS for the frontend
origin. Confirm that the browser can reach both the frontend and `/api/health`.
For HTTPS tunnel previews, Vite also supports `VITE_HMR_HOST` and
`VITE_HMR_CLIENT_PORT`; see [the Vite configuration](vite.config.ts).

## Build and validate

From the repository root:

```sh
pnpm --dir frontend test
pnpm --dir frontend lint
pnpm --dir frontend build
```

The build's preparation step generates icons and builds the shared widget package.
Use `dev:ee`, `build:ee`, or `test:ee` for the Enterprise edition. The default is
Community; its generated route tree excludes billing routes. See
[`package.json`](package.json) for browser-test and preview commands.

## Configuration and implementation

[Vite](vite.config.ts) enables the React Compiler through
`babel-plugin-react-compiler`, TanStack Router code splitting, and Tailwind CSS.
[ESLint](eslint.config.js) defines the project's lint rules.

Use the [architecture overview](../ARCHITECTURE.md) for frontend code entry points
and the [contribution guide](../CONTRIBUTING.md) for change requirements.
