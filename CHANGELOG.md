# Changelog

All important changes to this project are recorded in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Ginnungagap is now a pull-through proxy and cache for software packages ([ADR-0001](docs/explanation/adr/0001-rebuild-as-package-proxy.md)). The old malware investigation kit is stopped.
- The README describes the new project.

### Removed

- All code and configuration of the old malware investigation kit: the `isolator` sandbox, the Falco rules, the Go CLI, the honeypot script, and the Docker files. The new project does not use them.
