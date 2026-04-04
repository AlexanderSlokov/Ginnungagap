# Ginnungagap CLI (four-gees CLI)

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

`Ginnungagap` is a CLI tool that allows you to pre-check if your `npm install` is trying to plant something nasty
inside your computer or not.  

It takes your `package.*` (`.json` and `.lock`), bring it to an isolated container with a bunch of juicy credentials
that hackers are dreaming of (Kube config, Terraform .state, AWS credential, or your GEMINI_API_KEY, ...) 
`Falco` will watching `npm` run, and if there is something wrong, it will print out log that Ginnungagap can parse and order the container orchestrator (Docker / Podman) to pause and commit that container into a `tar.gz`. You can use that `tag.gz` for forensic,
submit it to `https://www.npmjs.com` so that they can take action, or yeet it on Reddit and make JS community cry again.

# Why do I need it?

Because you are too lazy to READ THE DAMN CVE REPORT before you install an `is-even package` that can "exec" command on your behalf!
(who even thinks that a package manager should exec script for a package???)

# Okay, I am all ears, explain how Ginnungagap works, seriously:

The architecture of `Ginnungagap` leverages container isolation and cloud-native runtime security to perform dynamic analysis 
(a.k.a. creating a honeypot) for your `npm` dependencies.

Here is the step-by-step lifecycle when you run `fourg run`:

1. **The Sandbox (`sandbox` container):** 
   Ginnungagap spins up an isolated, ephemeral Ubuntu container (`ubuntu:noble`). 
Your local `package.json` and `package-lock.json` are mounted into this container as **Read-Only**. 
Inside this sandbox, we plant fake "juicy credentials" (like a dummy `.kube/config`, `.aws/credentials`, etc.) 
to bait malicious post-install scripts. Then, it runs `npm install`.

2. **The Watcher (`falco` container):** 
   While the sandbox is running, an independent Falco container (running with `privileged` mode) monitors the host's kernel using eBPF/Syscall interception. 
Falco is configured with specific rules to detect suspicious activities generated *only* by the sandbox container. Examples include:
   - Attempting to read credentials.
   - Executing obfuscated shell commands (e.g., `base64 -d | sh`).
   - Making unexpected outbound network connections (e.g., curling a random IP to download a payload or exfiltrate data).
   - And so on. For short, "ANYTHING THAT A TEXT FILE SHOULD NOT DO".
   
   To maximize performance and bypass disk I/O bottlenecks, Falco is configured to run in **unbuffered mode (`-U`)** and outputs pure **JSON alerts directly to its `stdout`**.

3. **The Control Plane (`ginnungagap` CLI/Daemon):** 
   The Ginnungagap application acts as a high-speed Control Plane written in Go. Instead of relying on intermediate webhooks (like Falco Sidekick) or mounting log files, it **attaches directly to the Docker Socket** (`/var/run/docker.sock`).
   - It leverages the Docker Engine API to stream Falco's `stdout` in real-time (similar to `docker logs -f`).
   - A dedicated Goroutine continuously parses the JSON stream.
   - The precise millisecond it successfully unmarshals a JSON alert indicating a rule violation by the sandbox:
      - It immediately issues a **`docker pause`** command via the socket, freezing the sandbox container in its tracks before the malware can finish its execution or delete its tracks.
      - It then runs **`docker commit`** and exports the frozen state into a `.tar.gz` archive (password protected).
   - Finally, the CLI notifies you with a detailed forensic report, leaving you with the `.tar.gz` artifact to analyze, submit as a CVE, or share with the community.

# Installation

## Prerequisites

You need these tools presented on your computer first before you use `Ginnungagap CLI`:

1. A container engine, Docker is preferred than Podman.
2. `wget` or `curl`, `make` and `git`, to clone this repo or download the Go binary from GitHub release.

## Ubuntu / Debian like distro

## Windows

## MacOS