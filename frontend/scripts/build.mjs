import { spawnSync } from "node:child_process";

const edition = process.argv[2] ?? "community";
if (!["community", "ee"].includes(edition))
  throw new Error("Expected community or ee");
const env = {
  ...process.env,
  VITE_EDITION: edition,
  NODE_OPTIONS: "--max-old-space-size=4096",
};
const commands =
  edition === "ee"
    ? [
        ["tsc", "-p", "tsconfig.ee.json"],
        ["vite", "build"],
      ]
    : [
        ["tsc", "-b"],
        ["vite", "build"],
      ];
for (const [command, ...args] of commands) {
  const result = spawnSync(`node_modules/.bin/${command}`, args, {
    env,
    stdio: "inherit",
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}
