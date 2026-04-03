# Ginnungagap CLI (four-gees CLI)

![Bombastic Side Eyes Cat](bombastic_side_eyes_cat.png)

Let's see what is inside your package.json...
---

# 1. Who... are you?

I am an Infra guy, a guy that we usually call him "blue-collar servant for the DevOps and Dev teams." 

Most the time I hate `npm` in particular, and the whole JS ecosystem for short. But my dev team is keep using `npm` 
and force me to run `npm install --no-dev`. Hilariously in the past 6 months, there were 2 major supply chain attack
aim directly to `npm`, the `Shai-Hulud 2.0: The Return` and just right now, near April's fool: `Axios` outbreak. 

I CAN NOT TAKE IT ANYMORE! EVERYTIME I TOUCH JAVASCRIPT, SOMETHING BREAKS, SOMEWHERE!

Hence, I came with a conclusion: `npm` is a FLEA MARKET, with nothing to project its supply chain properly.

# Then what? WTF is this?

`Ginnungagap` is a CLI tool that allow you to pre-check if your `npm install` is trying to plant something nasty
inside your computer or not.  

It takes your `package.*` (`.json` and `.lock`), bring it in an isolated container with a bunch of juicy credentials
that hackers are dreaming of (Kube config, Terraform .state, AWS credential, or your GEMINI_API_KEY,...) 
`Falco container` will watching `npm` run, and if there is something wrong, `Falco Sidekick` will call out your container
orchestrator (Docker / Podman) to pause and commit that container into a `tar.gz`. You can use that `tag.gz` for forensic,
submit it to `https://www.npmjs.com` so that they can take action, or yeet it on Reddit and make JS community cry again.

# Why do I need it?

Because you are too lazy to READ THE DAMN CVE REPORT before you install an `is-even package` that can "exec" command on your behalf!
(who even think that package manager should exec script for package???)

# Installation

## Prerequisites

You need these tools presented on your computer first before you use `Ginnungagap CLI`:

1. A container engine, Podman is preferred than Docker due to non-root nature.
2. `wget` or `curl`, `make` and `git`, to clone this repo or download the Go binary from GitHub release.

## Ubuntu / Debian like distro

## Windows

## MacOS