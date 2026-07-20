# simple

<!-- TOC -->
* [simple](#simple)
  * [Memo](#memo)
<!-- TOC -->

## Memo

Profiling CPU usage.

```sh
$ valgrind --tool=callgrind --callgrind-out-file=/tmp/callgrind.out.%p ./target/debug/simple
$ set t (ls -t /tmp/callgrind.out.* | head -n1); kcachegrind $t; rm $t
```

Profiling Heap/Stack usage.

```sh
$ valgrind --tool=massif --massif-out-file=/tmp/massif.out.%p --heap=yes --stacks=yes ./target/debug/simple
$ set t (ls -t /tmp/massif.out.* | head -n1); massif-visualizer $t; rm $t
```

IntelliJ Rust does not support profiler.
