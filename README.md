# Ginnungagap - Supply Chain Attack Outbreak Investigation Kit

![Bombastic Side Eyes Cat](docs/images/bombastic_side_eyes_cat.png)

Let's see what is inside your package.json...
---

# 1. Who... are you?

I am an Infra guy, a guy that we usually call him "blue-collar servant for the DevOps and Dev teams." 

Most the time I hate `npm` in particular, and the whole JS ecosystem for short. But my dev team is keep using `npm` 
and force me to run `npm install --no-dev`. Hilariously in the past 6 months, there were two major supply chain attack
aim directly to `npm`, the `Shai-Hulud 2.0: The Return` and just right now, near April's fool: `Axios` outbreak. 

I CANNOT TAKE IT ANYMORE! EVERYTIME I TOUCH JAVASCRIPT, SOMETHING BREAKS, SOMEWHERE!

Hence, I came with a conclusion: `npm` is a FLEA MARKET, with nothing to project its supply chain properly.

# Then what? WTF is this?

`Ginnungagap` is a **Supply Chain Attack Outbreak Investigation Kit** or, on a larger scale, a **Mobile Diagnostic Laboratory ** designed exclusively for the Software Supply Chain.

We apply **Domain-Driven Design (DDD)** to extract technical requirements from real-world preventive medicine systems, transforming them into 1:1 equivalent software constraints. The ultimate goal: **Capture alive, freeze, and sequence the malware's genome** before it can self-destruct or spread.

# Okay, I am all ears, explain how Ginnungagap works, seriously:

The architecture of `Ginnungagap` leverages container isolation and cloud-native runtime security to perform dynamic analysis:

### 1. Containment Module (`sandbox` container)
This is the isolated environment where the malware is executed.
*   **Isolation Strictness**: Spins up a highly hardened `ubuntu:noble` container.
*   **Security Controls**: The container is strictly `read_only: true` with all capabilities dropped (`cap_drop: [ALL]`). The internal environment is completely sealed; data can only flow through controlled ports, preventing any escape to the Host.
*   **Network Throttling**: All outbound network traffic is intentionally throttled (Tarpitting) and monitored through virtual filters, ensuring malicious activities are delayed and trapped.

### 2. Catalyst/Honeypot Module (`generate_reagents.sh`)
This module is designed to elicit malicious behavior.
*   **Dynamic Honeypot**: Before the malicious payload runs, this module injects fake credentials (AWS keys, Kubeconfigs, `.env` secrets) generated randomly at runtime.
*   **Behavior Elicitation**: These dynamic honeypots force the malware to expose its data theft behaviors so we can capture the activity.

### 3. Behavioral Sequencer (`falco` container)
Instead of static file scanning, we analyze the runtime behavior of the processes:
*   **eBPF Tracing**: Hooks directly into the Kernel via Falco to monitor System Calls in real-time.
*   **Pattern Matching**: The moment a malicious syscall sequence is detected (e.g., `openat` on a sensitive file + `connect` to an unknown IP), the system triggers a JSON alert with ultra-low latency.

### 4. State Freezer & Archiver (`Go Control Plane`)
When malware is detected, it must be instantly frozen for research:
*   **Instant Freeze**: The Go CLI daemon intercepts the alert and directly writes to the Linux **Cgroup Freezer**, pausing the container in microseconds to prevent the malware from self-destructing.
*   **Archiving**: Dumps the entire frozen state of the container into a password-protected `.tar.gz` archive, ensuring a secure Chain of Custody when transferring the artifact for DFIR (Digital Forensics and Incident Response).

### 5. Sanitization Module
After testing is complete, all traces must be eradicated:
*   **100% Destruction**: Automatically force-kills, removes containers, and wipes out virtual networks and `tmpfs` volumes. Absolutely no dangling resources are left on the Host.

# Installation

## Prerequisites

You need these tools presented on your computer first before you use `Ginnungagap CLI`:

1. A container engine, Docker is preferred than Podman.
2. `wget` or `curl`, `make` and `git`, to clone this repo or download the Go binary from GitHub release.

## Ubuntu / Debian like distro

## Windows

## MacOS