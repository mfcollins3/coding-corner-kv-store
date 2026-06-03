# Key-Value Storage Engine

This repository contains the source code for a key-value storage engine. This
storage engine is based off of the article series published to
[The Coding Corner](https://read.thecoder.cafe/s/coding-corner),
part of [The Coder Cafe](https://read.thecoder.cafe/) on
[Substack](https://substack.com). The purpose of this project is to follow
along with the article series and share what I have built. I am completing the
article series just to re-learn forgotten concepts, learn new concepts, and to
learn from the author. I am sharing my source code so that others can see how
I approached and implemented each chapter of the article series.

## Required Software

- [Go 1.26.3](https://go.dev)

## Contents

1. [Week 1: In-Memory Store](#in-memory-store)
1. [Week 2: LSM Tree Foundations](v2/README.md)

## In-Memory Store

The initial implementation of the Key-Value Storage Engine uses a simple
in-memory map/hashtable/associative array where I am storing values indexed by
keys. I am supporting concurrency in this implementation by using a read/write
mutex that allows multiple concurrent readers and enforcing single-threaded
write access at any one time.
