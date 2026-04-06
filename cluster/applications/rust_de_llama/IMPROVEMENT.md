# Continuous Batching Improvements

Proposals for improving throughput and latency within the llama.cpp FFI boundary.
These changes target the Rust scheduling layer (`src/parallel.rs`, `src/parallel/slot.rs`)
without requiring upstream llama.cpp modifications.

## Background

The current implementation uses a fixed-slot batch processing loop:

1. Receive tasks from the channel or sleep for 1ms (`src/parallel.rs:307-329`)
2. Assign pending tasks to idle slots (`src/parallel.rs:331-343`)
3. Process all active slots in a single `process_batch` call (`src/parallel.rs:345-353`)

This design leaves GPU cycles on the table in three areas:
slots sit idle between iterations, long prompts block token generation for other slots,
and the fixed sleep interval adds unnecessary latency.

## Proposal 1: Chunked Prefill

**Priority: High** | **Difficulty: Medium** | **Impact: Latency stability**

### Problem

`decode_prompts()` (`src/parallel.rs:172-196`) processes an entire prompt in a single
`context.decode()` call. A 4096-token prompt monopolizes the GPU for that decode,
stalling token generation for all other active slots.

### Current Flow

```
Iteration N:
  [Slot 0: prompt 4096 tokens] → decode (blocks ~50ms)
  [Slot 1: waiting]            → generation delayed by ~50ms
  [Slot 2: waiting]            → generation delayed by ~50ms
```

### Proposed Change

Split prompt processing into chunks of `n_chunk` tokens (e.g., 256-512).
`slot.next_batch_tokens()` (`src/parallel/slot.rs:87-98`) already supports partial
draining via `cache_tokens.drain(..take_count)` with a `max_tokens` parameter.

The change is in `collect_batch_slots()` (`src/parallel.rs:357-380`):
interleave prompt chunks with generation tokens within the same batch,
respecting `n_batch` capacity.

```
Iteration N:
  [Slot 0: prompt chunk 256 tokens] + [Slot 1: generate 1 token] + [Slot 2: generate 1 token]
  → single decode call
Iteration N+1:
  [Slot 0: prompt chunk 256 tokens] + [Slot 1: generate 1 token] + [Slot 2: generate 1 token]
  → single decode call
```

### Implementation

1. Add a `max_prefill_chunk` field to `ParallelProcessor` (configurable, default 512)
2. In `collect_batch_slots()`, cap prompt tokens per slot to `max_prefill_chunk`
   instead of `remaining_capacity`
3. Remaining prompt tokens stay in `cache_tokens` (already a `VecDeque`)
   and are consumed in subsequent iterations
4. No changes needed to `slot.next_batch_tokens()` -- it already handles partial draining

### Tradeoff

Chunking increases total time-to-first-token for the chunked request itself.
A 4096-token prompt split into 256-token chunks requires 16 iterations instead of 1,
each with its own `context.decode()` kernel launch overhead. Later chunks also
attend to all previously cached KV entries, so per-chunk cost is not uniform.

This is a **latency fairness tradeoff**: the chunked request's TTFT increases
while other slots' generation latency becomes stable. In single-request scenarios
(no contention), chunked prefill is strictly worse. Consider bypassing chunking
when only one slot is active.

### Files to Modify

| File | Change |
|------|--------|
| `src/parallel.rs` | Add `max_prefill_chunk` field, modify `collect_batch_slots()` |
| `src/model_manager.rs` | Pass `max_prefill_chunk` from config |
| `src/config.rs` | Add `max_prefill_chunk` to model config |

## Proposal 2: KV Cache Reuse for Shared Prompts

**Priority: High** | **Difficulty: High** | **Impact: Prefill time reduction**

### Problem

Every request re-processes the system prompt from scratch, even when multiple requests
share identical system prompts. For a 512-token system prompt at 4 parallel slots,
this wastes 2048 tokens of redundant prefill per batch cycle.

### Proposed Change

Use `llama_memory_seq_cp` (formerly `llama_kv_cache_seq_cp`, renamed in the bundled
llama.cpp version, declared at `llama.cpp/include/llama.h:713`) to copy KV cache
entries from a reference sequence to new slots that share the same system prompt prefix.

### Implementation

1. Add a prompt prefix cache to `ParallelProcessor`:
   ```rust
   struct PrefixCacheEntry {
       prompt_hash: u64,
       sequence_id: i32,
       token_count: usize,
   }
   ```

2. In `assign_task_to_slot()` (`src/parallel.rs:198-258`):
   - Hash the system prompt portion of the token sequence
   - Look up the prefix cache for a matching entry
   - If found, call `llama_memory_seq_cp(memory, source_seq_id, target_seq_id, 0, token_count)`
     (note: first argument is `llama_memory_t`, not context)
   - Set `seq.n_past = token_count` and drain the matching prefix from `cache_tokens`
   - If not found, process normally and register the entry after prefill completes

3. Cache eviction: remove entries when no active slot uses the source sequence ID

### Constraints

- `llama_memory_seq_cp` must be exposed through the FFI layer (`src/lib.rs`).
  The function takes `llama_memory_t` (not `llama_context`), so a memory handle
  accessor must also be exposed.
- Sequence IDs map 1:1 to slot indices; the reference sequence needs a dedicated
  sequence ID outside the slot range (e.g., `n_parallel + cache_index`)
- The reference KV cache consumes context space, reducing available capacity.
  With the default `n_ctx = 2048`, a 512-token reference plus 4 active slots
  leaves only ~384 tokens per slot for generation. Consider reserving a fraction
  of `n_ctx` for reference sequences and failing gracefully when exceeded.
- Unbounded reference sequences can exhaust the KV cache. Limit the number of
  cached prefixes (e.g., max 2-4 entries) and evict least-recently-used entries.

### Security Considerations

The reference sequence's KV cache must be treated as **read-only** after prefill.
If the reference sequence ID is reused for generation, the attention key/value vectors
would encode user-specific context, which then leaks into subsequent requests that
copy from it. Mitigation:

- Use a dedicated sequence ID range that is never assigned to active generation slots
- Clear the reference KV cache if any non-prefill tokens are accidentally decoded on it
- Use full token-sequence comparison (not just hash) for cache lookup to prevent
  hash collision from silently sharing KV cache between unrelated prompts

### Files to Modify

| File | Change |
|------|--------|
| `src/lib.rs` | Expose `llama_memory_seq_cp` and memory handle accessor via FFI |
| `src/parallel.rs` | Add `PrefixCacheEntry`, modify `assign_task_to_slot()` |
| `src/parallel/slot.rs` | Add `skip_prefix()` method to drain cached prefix tokens |

## Proposal 3: Immediate Slot Reuse

**Priority: Medium** | **Difficulty: Medium** | **Impact: Slot utilization**

### Problem

When a slot completes generation in `generate_tokens()` (`src/parallel.rs:488-503`),
it becomes idle but is not refilled until the next iteration of the main loop
(`src/parallel.rs:331-343`). This wastes one full iteration cycle per completed slot.

### Current Flow

```
Iteration N:
  generate_tokens() → Slot 2 completes → slot.stop_task()
  (loop ends, no new task assigned)
Iteration N+1:
  (1ms sleep or channel recv)
  assign pending task to Slot 2  ← wasted iteration
  process_batch()
```

### Proposed Change

After `generate_tokens()` completes, immediately check for pending tasks
and assign them to newly freed slots before returning to the main loop.

### Implementation

1. Change `run_processing_loop()` to pass `&mut pending_tasks` to `process_batch()`
   (or a new `post_generation_assign()` method)
2. After `slot.stop_task()` in `generate_tokens()` (`src/parallel.rs:501`),
   call a slot assignment function that pops from `pending_tasks`
3. The newly assigned slot participates in the next `process_batch()` call
   without waiting for the main loop to cycle

### Borrow Constraint

`generate_tokens()` (`src/parallel.rs:382`) takes `&mut [Slot]`. Calling
`assign_task_to_slot()` also requires `&mut [Slot]`. These cannot overlap
in the same scope. The assignment must happen **after** `generate_tokens()` returns,
not inside it.

The recommended approach is to extract the assignment logic into a standalone method
and call it in `run_processing_loop()` immediately after `process_batch()` returns:

```rust
// In run_processing_loop(), after process_batch():
self.assign_pending_tasks(&mut slots, &mut pending_tasks).await;

// Extracted method:
async fn assign_pending_tasks(
    &self,
    slots: &mut [Slot],
    pending_tasks: &mut VecDeque<Task>,
) -> Result<(), error::Error> {
    while !pending_tasks.is_empty() {
        if let Some(slot_id) = Self::get_available_slot(slots) {
            let task = pending_tasks.pop_front().unwrap();
            self.assign_task_to_slot(slot_id, task, slots).await?;
        } else {
            break;
        }
    }
    Ok(())
}
```

This avoids the borrow conflict while still reusing slots within the same
iteration (before the loop cycles back to `tokio::select!`).

### Files to Modify

| File | Change |
|------|--------|
| `src/parallel.rs` | Extract `assign_pending_tasks()`, call after `generate_tokens()` |

## Proposal 4: Adaptive Batch Interval

**Priority: Low** | **Difficulty: Low** | **Impact: Marginal throughput**

### Problem

The main loop uses a fixed 1ms sleep (`BATCH_INTERVAL_MS = 1` at `src/parallel.rs:26`)
regardless of load. When all slots are actively generating, this 1ms is pure overhead.
When no slots are active, the loop blocks on `task_rx.recv()` anyway.

### Proposed Change

Remove the fixed sleep when active work exists. The current `tokio::select!`
(`src/parallel.rs:312-318`) already handles the idle case by blocking on `recv()`.

### Implementation

Replace the `tokio::select!` block:

```rust
// Before
if has_pending_work {
    tokio::select! {
        Some(task) = task_rx.recv() => {
            pending_tasks.push_back(task);
        }
        _ = tokio::time::sleep(batch_interval) => {
        }
    }
}

// After
if has_pending_work {
    match task_rx.try_recv() {
        Ok(task) => pending_tasks.push_back(task),
        Err(tokio::sync::mpsc::error::TryRecvError::Empty) => {}
        Err(tokio::sync::mpsc::error::TryRecvError::Disconnected) => break,
    }
}
```

`try_recv()` is non-blocking: if a new task is available it is consumed immediately,
otherwise the loop proceeds directly to `process_batch()` without sleeping.

### Tradeoff

- Removes 1ms of idle time per iteration when slots are active
- **CPU spin cost**: the processing loop runs in a dedicated `std::thread`
  (`src/model_manager.rs:146-153`) with its own Tokio runtime. A tight loop
  consumes one full CPU core continuously, even while `context.decode()` blocks
  the GPU. If decode takes 10-50ms, the 1ms saving is a 2-10% improvement
  at the cost of 100% CPU utilization on that core between decodes.
- **CPU-only inference (no `cuda` feature)**: decode itself uses CPU threads via
  OpenMP. A busy-wait loop on the scheduling thread competes for CPU time with
  inference threads, potentially **degrading** throughput. For CPU-only builds,
  a shorter sleep (e.g., 100us) is required, not optional.

### Files to Modify

| File | Change |
|------|--------|
| `src/parallel.rs` | Replace `tokio::select!` with `try_recv()` when `has_pending_work` |

## Summary

| # | Proposal | Priority | Difficulty | Primary Metric |
|---|----------|----------|------------|----------------|
| 1 | Chunked Prefill | High | Medium | Latency P99 during concurrent prefill+generation |
| 2 | KV Cache Reuse | High | High | Prefill time for repeated system prompts |
| 3 | Immediate Slot Reuse | Medium | Medium | Slot utilization under sustained load |
| 4 | Adaptive Batch Interval | Low | Low | Tokens/sec at full slot occupancy |

Proposals 1 and 4 are independent and can be implemented in parallel.
Proposal 3 is a prerequisite for maximizing the benefit of Proposal 1
(chunked prefill frees slots more gradually, making immediate reuse more impactful).
Proposal 2 is independent but requires FFI changes.
