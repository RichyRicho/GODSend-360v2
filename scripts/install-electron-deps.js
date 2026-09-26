#!/usr/bin/env node

const { spawnSync } = require("child_process");
const path = require("path");

const root = path.resolve(__dirname, "..");
const electronDir = path.join(root, "src", "electron-app");

const npmCommand = process.platform === "win32" ? "npm.cmd" : "npm";

console.log("[install-electron-deps] Installing Electron dependencies");

const result = spawnSync(npmCommand, ["install", "--prefix", electronDir], {
  cwd: root,
  stdio: "inherit",
  env: process.env,
  shell: false,
});

if (result.error) {
  console.error("[install-electron-deps] Failed to launch npm:", result.error.message);
  process.exit(1);
}

process.exit(result.status ?? 1);
