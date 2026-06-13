# Week 4: Deletes, Tombstones, and Compaction

This module implements the
[week 4 challenge](https://read.thecoder.cafe/p/build-your-own-kv-engine-4).
This challenge adds a new `DELETE` operation that will mark a key as deleted
by writing a tombstone to the log. The tombstone is a special value that
indicates that the key has been deleted. When the engine encounters a
tombstone, it will treat the key as if it does not exist.

This challenge also implements compaction of the SSTables. Compaction is the
process of mergin multiple SSTables into a single SSTable. This is necessary
because as the engine writes more and more SSTables, it will become slower to
read from them. Compaction helps to improve the read performance of the engine
by reducing the number of SSTables that need to be read. Compaction happens
after every 10,000 `PUT` or `DELETE` operations. During compaction, the engine
will use a k-way merge algorithms with a min heap to merge the SSTables. Any
tombstones that are encountered during the merge will be discarded, and any
keys that are marked as deleted will not be included in the final SSTable.

I also refactored the `kvstore` module in this challenge to make ti more
maintainable and easier to find the code. The compaction algorithm was
implemented in [`compaction.go`](internal/kvstore/compaction.go). The
write-ahead log code was moved to the [`wal.go`](internal/kvstore/wal.go)
file. The SSTable code was moved to the 
[`sstable.go`](internal/kvstore/sstable.go) file.

