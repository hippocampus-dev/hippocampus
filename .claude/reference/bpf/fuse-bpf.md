# fuse-bpf struct_ops Filters and Daemons

What the fuse-bpf kernel contract does and does not do for a `SEC("struct_ops/...")` program attached to `struct fuse_ops`, and for the daemon that answers the requests it routes.
Each item below is checked against the tree `kernel-lab/fuse-bpf/experiment.sh` builds, under `kernel-lab/.build/fuse-bpf/linux/`.

Example: `kernel-lab/fuse-bpf/masknas/src/bpf/mask.bpf.c` and its daemon `kernel-lab/fuse-bpf/masknas/src/main.rs`

## The Backing Path Never Assigns a nodeid

`fuse_lookup_backing()` fills only `feo->attr`, through `fuse_stat_to_attr()`.
It never writes `feo->nodeid`, and the dispatch macro zeroes the reply struct before the call, so a filter that keys a map on `out->nodeid` keys every entry on 0.
Nothing reports this: the program compiles, the verifier accepts it, and the map simply collapses to one entry.

`fuse_lookup_finalize()` then does `get_fuse_inode(inode)->nodeid = feo->nodeid;`, so whatever the reply carries becomes the inode's nodeid.
Assigning it is the daemon's job, and a filter that wants to recognise the inode later keys its map on `out->attr.ino` — `out->nodeid` is not populated yet when the postfilter runs.

Do not treat a non-zero nodeid as the mark itself.
`fuse_mknod_backing()`, `fuse_mkdir_backing()`, `fuse_link_backing()` and `fuse_symlink_backing()` all call `fuse_iget_backing(dir->i_sb, get_fuse_inode(dir)->nodeid, dir's backing_inode)`, so a child created through the mount inherits the *parent directory's* nodeid — and at the mount root, where the parent was hashed on that same nodeid, `iget5_locked` hands back the parent inode itself.
Any scheme that reads meaning into "nodeid is non-zero" therefore mistakes those children for whatever their parent was, and the daemon resolves them to the parent's path.
Nothing in such a request distinguishes it from one made directly in the parent, so rebuilding the path from the request's nodeid and checking that path's inode number against it necessarily matches: that check catches a stale name, not this.
At the root it cannot even run, since `FUSE_ROOT_ID` is a protocol constant with no backing inode number to compare against.

Why the root alone hands back the parent inode is worth spelling out, because the opposite conclusion is the easy one to reach.
`fuse_iget_backing()` hashes on the backing inode pointer unless the nodeid it was given is non-zero, and an ordinary lookup passes 0 and writes the nodeid in afterwards — so a directory learned through lookup does not live in its own nodeid's bucket, and the child's `hash = parent nodeid` misses it.
The root does live there, having been hashed on nodeid 1 from the start.
Below the root the same hashing makes mount-created siblings collide with *each other* instead: the first child lands in the parent nodeid's bucket, and the second matches it on both nodeid and backing inode.

A reserved bit of the nodeid can still carry a mark, because the kernel only hashes and compares the value — `fuse_iget_backing()` takes `hash = nodeid` whenever it is non-zero, `fuse_inode_backing_eq()` compares it whole, and `fuse_open_finalize()` copies it into `ff->nodeid` — so the bit survives into a read prefilter's `meta->nodeid`.
Set it only on nodeids no child can inherit: leaving every directory's nodeid untagged makes the value a mount-created child inherits read as unmarked.
To notice the collision at all, wire `mkdir_prefilter`/`mkdir_postfilter` and return `BPF_FUSE_USER_POSTFILTER` — the request carries the parent's nodeid, which is the only notice the daemon gets that this nodeid now names two directories, and `fuse_mkdir_initialize_out()` leaves `out_numargs` at 0 so the reply is header-only.
`mknod`, `link` and `symlink` inherit the same way and wiring mkdir alone does not cover them.

`FUSE_READ` keys on the *handle's* nodeid, and only `fuse_open_finalize()` fills that in; `fuse_create_open_finalize()` sets `fi->nodeid` and `ff->fh` but leaves `ff->nodeid` at the 0 `fuse_file_alloc()` gave it.
So even a correctly marked inode is invisible to a read prefilter through the handle its own create returned.

`fuse_create_open_backing()` calls `fuse_iget_backing(sb, 0, ...)` and never runs a lookup, and `fuse_create_open_initialize_out()` zeroes both reply structs, so a `create_open` postfilter sees `entry_out.nodeid` and `entry_out.attr` all-zero.
The `out->attr.ino` key the lookup postfilter falls back on does not exist on this path, so a filter that copies that workaround into `create_open` silently keys every created file on 0.
`fuse_create_open_finalize()` still does `fi->nodeid = feo->nodeid`, so the create reply is what installs the nodeid — but only userspace can supply one, by stat'ing the backing file itself.

## A Failed Lookup Still Reaches the Postfilter

On a negative dentry `fuse_lookup_backing()` sets `fa->info.error_in = -ENOENT` and returns before `fuse_stat_to_attr()`.
The reply struct is zeroed rather than left undefined, so the hazard is a bogus all-zero `attr`, not undefined behaviour — a filter that reads `out->attr` without checking `meta->error_in` first will happily record inode 0.

Check `meta->error_in` before touching anything derived from `out`.

## A Negative Prefilter Return Aborts the Operation

The slots in `struct fuse_ops` are declared `uint32_t`, but the dispatch macro assigns the result to an `int` and tests `if (bpf_next < 0)`, propagating it as the operation's error.
So a prefilter can fail an operation closed by returning `-ESOMETHING` even though its signature says unsigned; the verifier does not range-check a non-void `BPF_PROG_TYPE_STRUCT_OPS` return.
A prefilter's negative return breaks out before `backing()` runs and before `initialized` is set, so `finalize()` never runs either — see `## A Postfilter Failure Does Not Undo the Operation` for why the same return from a postfilter is not an abort.

## A Postfilter Failure Does Not Undo the Operation

The dispatch macro calls `backing()` before it raises `FUSE_POSTFILTER`, and it treats a negative filter return and a negative daemon reply identically: `error` is set, the loop breaks, `finalize()` still runs, and nothing rolls the backing operation back.
So on a mutating op — `create_open` above all — a daemon that answers a postfilter with an error fails the caller's syscall while the backing file it just created stays on disk.

Where the daemon cannot do what the postfilter asked of it, echo the out structs back unmodified instead.
That is byte for byte what `BPF_FUSE_CONTINUE` would have produced, so the operation completes with the values the backing path filled in.
An error reply is the honest answer only for a read-only op such as `lookup` or `read`, where nothing has changed.

## A Postfilter Runs Only If the Prefilter Asked For It

The dispatch macro calls `call_postfilter` only when the prefilter returned `BPF_FUSE_POSTFILTER`, and it breaks out entirely on `BPF_FUSE_CONTINUE` before reaching that point.
An unset prefilter slot yields `BPF_FUSE_CALL_DEFAULT`, which becomes `BPF_FUSE_CONTINUE` when no `default_filter` is set, so wiring only `*_postfilter` in `struct fuse_ops` produces a program that loads, attaches and never fires.
A filter whose work is entirely in the postfilter therefore still needs a prefilter whose only body is `return BPF_FUSE_POSTFILTER;`.

`fuse_mkdir_postfilter()` guards on `ops->mkdir_prefilter` before calling `ops->mkdir_postfilter`, the one such mismatch among that file's postfilter dispatchers, so setting `mkdir_prefilter` alone has the kernel call a NULL pointer rather than fall back to `BPF_FUSE_CALL_DEFAULT`.
Wire both mkdir slots or neither.

## Forget Arrives for Inodes No Filter Routed

`fuse_iget_backing()` bumps `fi->nlookup` on every call, including the ones the backing path makes when the prefilter returned `BPF_FUSE_CONTINUE` and no request ever reached the daemon.
`fuse_evict_inode()` then queues one forget carrying the accumulated count and `fi->nodeid`, so a daemon holding a nodeid-keyed table is told to forget nodeids it never learned, with counts larger than the lookups it saw.
Because `mknod`, `mkdir`, `link` and `symlink` children inherit the parent directory's nodeid, such a forget can also carry a live parent's nodeid.

## A Cached dentry Is Never Re-Looked-Up

`fuse_dentry_revalidate()` returns valid immediately when the parent has a backing inode, without issuing a lookup.
So whatever a lookup filter decided about a name holds until the dentry falls out of the dcache — a mark dropped in between is not re-established, and a stale one is not corrected.
