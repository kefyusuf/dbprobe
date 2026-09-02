# Modernc final acceptance trigger

This commit exists only to trigger the final fail-closed acceptance chain after the self-review bootstrap has had time to finish.

The branch must not be integrated unless the permanent evidence gate passes:

- exact modernc module pin;
- live SQLite close/reopen and conflict tests;
- full Go 1.25 `make ci`;
- CGo-free Linux, Windows, and macOS builds;
- MySQL 8.0.46 and 8.4.11 Docker acceptance.
