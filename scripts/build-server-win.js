#!/usr/bin/env node
const { spawnSync } = require("child_process");
const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const dist = path.join(root, "dist");
const out = path.join(dist, "godsend.exe");
const serverDir = path.join(root, "src", "server");

fs.mkdirSync(dist, { recursive: true });

console.log("\n[build-server-win] Removing stale Windows backend if present");
try {
  if (fs.existsSync(out)) fs.rmSync(out, { force: true });
} catch (e) {
  console.error("[build-server-win] Could not remove stale godsend.exe:", e.message);
  process.exit(1);
}

console.log("\n[build-server-win] Building windows/amd64 -> dist/godsend.exe");
const r = spawnSync("go", ["build", "-o", out, "."], {
  cwd: serverDir,
  stdio: "inherit",
  env: { ...process.env, GOOS: "windows", GOARCH: "amd64", CGO_ENABLED: "0" },
  shell: false,
});

if (r.error) {
  console.error("[build-server-win] Go spawn failed:", r.error.message);
  process.exit(1);
}
if (r.status !== 0) process.exit(r.status ?? 1);

console.log("\n[build-server-win] Verifying Windows binary");
const verify = path.join(__dirname, "verify-go-binaries.js");
const v = spawnSync(process.execPath, [verify, "windows"], {
  stdio: "inherit",
  cwd: root,
  env: process.env,
});
if (v.status !== 0) process.exit(v.status ?? 1);

console.log("\n[build-server-win] Done.");
