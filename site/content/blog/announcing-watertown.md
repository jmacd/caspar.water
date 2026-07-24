---
title: Announcing Watertown
weight: 60
layout: blog
section: Blog
date: "2026-07-01"
image: "/img/watertown1.svg"
---

[Watertown](https://github.com/jmacd/watertown) is new software
written by the Caspar Water Company.

Watertown is built as a layered stack of Rust crates on top of the
Apache Arrow, DataFusion, and Delta Lake ecosystem. Each layer builds
on the ones below it, and the top-level `cmd` crate links everything
into a single `pond` binary.

![How Watertown is built](/img/watertown-architecture.svg)
