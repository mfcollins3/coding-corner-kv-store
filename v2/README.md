# Week 2: LSM Tree

This module implements the week 2 assignment for the 
[Build Your Own Key-Value Storage Engine](https://read.thecoder.cafe/p/build-your-own-kv-engine-2)
project. It includes the implementation of a Log-Structured Merge Tree (LSM 
Tree). The LSM Tree is a data structure that provides efficient write and read
operations by organizing data into multiple levels of sorted runs.

For the LSM Tree implementation, I changed the key-value storage engine to
use a **Memtable**, which is similar to the in-memory data structure used in
the previous version. The Memtable collects key-value pairs in memory until
it reaches a size threshold (2000 pairs). When the size threshold is reached,
Memtable will flush its contents to disk as a new sorted run, called an
SSTable. Each SSTable is stored as a separate file on disk, and the LSM Tree
maintains a list of these SSTables in a file named `MANIFEST`.

On read operations, the LSM Tree first checks the Memtable for the requested
key. If the key is not found in the Memtable, it then searches through the
SSTables in the order they were created (newest to oldest) until it finds the
key or exhausts all SSTables. This approach allows for efficient reads while
also optimizing for write performance by minimizing disk I/O.

To improve performance, I implemented both optional exercises. I have
implemented a negative cache to store keys that have been searched for but not
found in either the Memtable or any of the SSTables. This cache helps to avoid
unnecessary disk reads for keys that are known to be absent.

I also implemented a `MANIFEST` cache, which is basically a list of the
SSTables that have been created. This cache avoids the need to read the
`MANIFEST` file from disk on every read operation, improving the efficiency of
the LSM Tree.
