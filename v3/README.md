# Week 3: Durability with Write-Ahead Logging

This module implements the
[week 3 challenge](https://read.thecoder.cafe/p/build-your-own-kv-engine-3) 
to add write-ahead logging to the Key-Value Storage Engine for durability. When
new items are added to the Key-Value Storage Engine using `PUT` operations, the
updates are first written and committed to disk in a write-ahead log. This
durability ensures that no changes are lost if the service crashes before the
memtable is committed to disk as an SSTable. When the service starts, the
memtable is initialized by replaying the operations in the write-ahead log.

After getting the write-ahead log working, I added a CRC32 checksum to the log
entries. I added code to calculate the CRC32 checksum before writing the log
entry to the write-ahead log. I also added code to validate the CRC32 checksum
when replaying the log and reporting an error if the on-disk data has become
corrupted.

My initial implementation used the `File.Sync` operation to persist writes to
disk for the write-ahead log. I changed this at the end and instead opened the
write-ahead log file using the `syscall.O_DSYNC` flag which automatically
flushes the data to disk and only the necessary file metadata (file size, disk
information) to disk. The use of the `syscall.O_DSYNC` flag significantly
improved the performance because less metadata needs to be written for the file.
