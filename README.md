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

## Instructions

Start by cloning my GitHub repository. You can use the
[GitHub CLI](https://cli.github.com) to clone it:

```bash
gh repo clone mfcollins3/coding-corner-kv-store
```

Or you can use Git:

```bash
git clone https://github.com/mfcollins3/coding-corner-kv-store.git
```

The first step that you will need is to generate test data. You can run the
`gen` program to generate the data:

```bash
go run ./cmd/gen put 30000
```

This command will run `gen` to generate 30,000 PUT/GET records for testing and
will output them to a `put.txt` file.

Next, start the Key-Value Storage Engine. In a separate terminal, run:

```bash
go run ./cmd/server
```

This command will start the server and it will listen at https://localhost:8080.
The listening port can be changed by setting the `PORT` environment variable:

```bash
PORT=32000 go run ./cmd/server
```

Finally, back in the first terminal, run the client program. This program will
read the `put.txt` file and will execute the requests against the server:

```bash
go run ./cmd/client
```

You can also manually test the Key-Value Storage Engine using `curl`. To store
a value, run:

```bash
curl -X PUT http://localhost:8080/kv/hello -d 'World!'
```

The URL pattern for the Key-Value Storage Engine is:

```plain
http://localhost:8080/kv/{key}
```

For the `PUT` request, the value is uploaded in the body of the request. To
retrieve the stored value, run:

```bash
curl http://localhost:8080/kv/hello
```

If you want to see the latency metrics from the server, run:

```bash
curl http://localhost:8080/metrics
```

The `/metrics` endpoint will return the p50, p95, and p99 latency percentiles
from the Key-Value Storage Engine:

- **p50**: The median latency, in milliseconds. Half of all requests are faster
  than this time.
- **p95**: The upper tail of latency, in millseconds. 95% of all requests are
  faster than this time.
- **p99**: The critical tail of latency, in milliseconds. 99% of requests are
  faster than this time.
