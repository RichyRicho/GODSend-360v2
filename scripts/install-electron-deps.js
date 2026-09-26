#!/usr/bin/env node

const { spawnSync } = require("child_process");
const path = require("path");

const root = path.resolve(__dirname, "..");
const electronDir = path.join(root, "src", "electron-app");
const npmExecPath = process.env.npm_execpath;

if (!npmExecPath) {
  console.error("[install-electron-deps] npm_execpath is not set");
  process.exit(1);
}

console.log("[install-electron-deps] Installing Electron dependencies using the invoking npm CLI");

const result = spawnSync(process.execPath, [npmExecPath, "install", "--prefix", electronDir], {
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
