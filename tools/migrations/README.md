# Special migration tools

This directory contains special-purpose or one-shot migration tools. They are
not part of the normal agent-compose upgrade path and are intentionally not
wired into Task or CI entry points.

`resource-identity-one-shot-reset.sh` moves the agent-compose database and
runtime state into a timestamped backup so a new daemon can initialize fresh
state. Normal upgrades should use the installer's compatibility flow. Use this
reset only when incompatible state must be reinitialized explicitly, and run
it with `--dry-run` before making changes.
