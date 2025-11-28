# Actor Model Refactoring Roadmap

## Overview

This document provides a comprehensive plan for refactoring `parallel.rs` from a polling-based architecture to an actor-based architecture using message passing.

**Status**: Proposal (Not Yet Implemented)
**Created**: 2025-10-30
**Estimated Effort**: 3-5 days
**Priority**: Low (Future Enhancement)

---

## Executive Summary

### Current State (Polling-Based)
- **Lines of Code**: 762 lines in `parallel.rs`
- **Architecture**: Single `run_processing_loop` with 7 responsibilities
- **Strengths**: Maximum performance, simple debugging, natural fit with llama.cpp
- **Weaknesses**: Tight coupling, difficult to extend, hard to unit test

### Target State (Actor-Based)
- **Lines of Code**: ~350 lines (split across 4 files)
- **Architecture**: Message-passing between SequenceActor and BatchEngine
- **Strengths**: Separation of concerns, easy to extend, unit testable, ~50% less code
- **Weaknesses**: Slightly harder debugging, requires understanding async messaging

### Key Trade-offs

| Aspect | Current (Polling) | Actor Model | Winner |
|--------|------------------|-------------|--------|
| Performance | ⭐⭐⭐⭐⭐ (0.4-0.8% CPU) | ⭐⭐⭐⭐ (~0.5-1% CPU, +0.1% overhead) | Current |
| Code Size | 762 lines | ~350 lines | **Actor** |
| Maintainability | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | **Actor** |
| Extensibility | ⭐⭐ | ⭐⭐⭐⭐⭐ | **Actor** |
| Testability | ⭐⭐ (integration only) | ⭐⭐⭐⭐⭐ (unit + integration) | **Actor** |
| Debug Simplicity | ⭐⭐⭐⭐ (stack traces) | ⭐⭐⭐ (message flow tracking) | Current |

---

## When to Implement This

### ✅ Implement If:
1. **Frequent feature additions** - New sampling strategies, stop conditions, etc.
2. **Maintenance difficulties** - Bugs are hard to find, changes break unexpectedly
3. **Testing requirements** - Need unit tests for components
4. **Multiple contributors** - Team needs clear separation of concerns

### ❌ Don't Implement If:
1. **Current code works fine** - No maintenance pain
2. **No upcoming features** - Static requirements
3. **Performance is critical** - Every 0.1% CPU matters
4. **Small team/solo developer** - Can navigate current complexity

---

## Architecture Design

### High-Level Overview

```
┌─────────────────┐
│ HTTP Handler    │
│ (chat_compl.rs) │
└────────┬────────┘
         │ submit_task(Task)
         ▼
┌─────────────────────────────────────────┐
│ Coordinator                             │
│ - Spawns SequenceActors                │
│ - Manages actor lifecycle               │
└────────┬───────────────────────┬────────┘
         │                       │
         │ spawn                 │ batch_rx
         ▼                       ▼
┌──────────────────┐    ┌────────────────────┐
│ SequenceActor    │───→│ BatchEngine        │
│ (per request)    │    │ (singleton)        │
│                  │    │                    │
│ - State mgmt     │    │ - Collect requests │
│ - Stop detection │◄───│ - llama_decode()   │
│ - Token output   │    │ - Distribute results│
└──────────────────┘    └────────────────────┘
     request_tx              result_rx
```

### Component Breakdown

#### 1. SequenceActor (`src/sequence_actor.rs`, ~150 lines)

**Responsibilities:**
- Manage single request lifecycle
- Maintain generation state (n_past, tokens, stop conditions)
- Request token generation from BatchEngine
- Check stop conditions
- Send tokens to client

**Key Structure:**
```rust
pub struct SequenceActor {
    id: SequenceId,
    task: Task,
    state: GenerationState,
    stop_matcher: StopMatcher,
    request_tx: Sender<DecodeRequest>,
    result_rx: Receiver<DecodeResult>,
    output_tx: Sender<TaskResponse>,
}

impl SequenceActor {
    pub async fn run(mut self) {
        // Main loop: ~20 lines
        loop {
            self.request_next_token().await;
            let token = self.receive_result().await;
            if self.should_stop(token) { break; }
            self.send_to_client(token).await;
        }
    }
}
```

**Advantages:**
- Small, focused component (~20 line main loop)
- Easy to unit test
- Can add new features without affecting other parts

#### 2. BatchEngine (`src/batch_engine.rs`, ~100 lines)

**Responsibilities:**
- Collect decode requests from multiple actors
- Build llama_batch
- Call llama_decode() once for all requests
- Distribute results back to actors

**Key Structure:**
```rust
pub struct BatchEngine {
    model: Arc<LlamaModel>,
    context: LlamaContext,
    batch_buffer: BatchBuffer,
    request_rx: Receiver<DecodeRequest>,
}

impl BatchEngine {
    pub async fn run(mut self) {
        loop {
            // Collect requests with timeout
            let requests = self.collect_requests(Duration::from_millis(1)).await;

            // Build batch
            let batch = self.build_batch(&requests);

            // Decode (single llama.cpp call)
            self.context.decode(batch)?;

            // Distribute results
            self.send_results(&requests).await;
        }
    }
}
```

**Key Design Decision: Batching Strategy**

```rust
async fn collect_requests(&mut self, timeout: Duration) -> Vec<DecodeRequest> {
    let mut requests = Vec::new();
    let deadline = Instant::now() + timeout;

    // Option A: Fixed timeout (recommended for start)
    while Instant::now() < deadline {
        if let Ok(req) = self.request_rx.try_recv() {
            requests.push(req);
        }
    }

    // Option B: Dynamic (future optimization)
    // Adjust timeout based on load

    requests
}
```

#### 3. Coordinator (`src/coordinator.rs`, ~50 lines)

**Responsibilities:**
- Spawn SequenceActor for each incoming task
- Create communication channels
- Cleanup completed actors

**Key Structure:**
```rust
pub struct Coordinator {
    batch_engine_tx: Sender<DecodeRequest>,
    actors: HashMap<SequenceId, JoinHandle<()>>,
}

impl Coordinator {
    pub fn submit_task(&mut self, task: Task) -> Receiver<TaskResponse> {
        let (output_tx, output_rx) = mpsc::channel(100);
        let (result_tx, result_rx) = mpsc::channel(1);

        let actor = SequenceActor::new(
            task,
            self.batch_engine_tx.clone(),
            result_rx,
            output_tx,
        );

        let handle = tokio::spawn(actor.run());
        self.actors.insert(task.id, handle);

        output_rx
    }
}
```

#### 4. Shared Types (`src/actor_types.rs`, ~50 lines)

```rust
pub struct DecodeRequest {
    pub seq_id: SequenceId,
    pub token: i32,
    pub n_past: i32,
    pub result_tx: oneshot::Sender<DecodeResult>,
}

pub struct DecodeResult {
    pub seq_id: SequenceId,
    pub logits: Vec<f32>,
}

pub struct GenerationState {
    pub n_past: i32,
    pub generated_tokens: Vec<i32>,
    pub cache_tokens: VecDeque<i32>,
}
```

---

## Implementation Plan

### Phase 0: Preparation (1-2 hours)

**Goal**: Set up parallel implementation without breaking current code

```bash
# Create new files alongside existing parallel.rs
touch src/sequence_actor.rs
touch src/batch_engine.rs
touch src/coordinator.rs
touch src/actor_types.rs

# Add feature flag to Cargo.toml
[features]
actor-model = []
```

**Test**: Compile succeeds, existing tests pass

---

### Phase 1: Implement Core Types (2-3 hours)

**File**: `src/actor_types.rs`

**Tasks**:
1. Define `DecodeRequest`, `DecodeResult`, `GenerationState`
2. Define `SequenceId` type alias
3. Add documentation for each type

**Test**:
```rust
#[cfg(test)]
mod tests {
    #[test]
    fn test_types_compile() {
        let req = DecodeRequest { /* ... */ };
        assert_eq!(req.seq_id, 0);
    }
}
```

---

### Phase 2: Implement BatchEngine (4-6 hours)

**File**: `src/batch_engine.rs`

**Implementation Order**:
1. Basic structure + constructor
2. `collect_requests()` with fixed 1ms timeout
3. `build_batch()` from requests
4. `decode()` wrapper around llama.cpp
5. `send_results()` distribution

**Key Code Sections**:

```rust
impl BatchEngine {
    async fn collect_requests(&mut self, timeout: Duration) -> Vec<DecodeRequest> {
        let mut requests = Vec::new();
        let deadline = Instant::now() + timeout;

        loop {
            let remaining = deadline.saturating_duration_since(Instant::now());
            if remaining.is_zero() {
                break;
            }

            tokio::select! {
                Some(req) = self.request_rx.recv() => {
                    requests.push(req);
                }
                _ = tokio::time::sleep(remaining) => {
                    break;
                }
            }
        }

        requests
    }

    fn build_batch(&mut self, requests: &[DecodeRequest]) -> Result<(), Error> {
        self.batch_buffer.reset();

        for req in requests {
            self.batch_buffer.add_token(
                req.token,
                req.n_past,
                req.seq_id as i32,
                1, // logits
            );
        }

        let batch = self.batch_buffer.as_llama_batch();
        self.context.decode(batch)?;

        Ok(())
    }

    fn send_results(&self, requests: &[DecodeRequest]) -> Result<(), Error> {
        for req in requests {
            let logits = self.context.get_logits_ith(req.seq_id as i32)?;

            let result = DecodeResult {
                seq_id: req.seq_id,
                logits: logits.to_vec(),
            };

            // oneshot channel, ignore if receiver dropped
            let _ = req.result_tx.send(result);
        }

        Ok(())
    }
}
```

**Test**:
```rust
#[tokio::test]
async fn test_batch_engine_single_request() {
    let (tx, rx) = mpsc::unbounded_channel();
    let engine = BatchEngine::new(model, rx);

    // Spawn engine
    let handle = tokio::spawn(engine.run());

    // Send request
    let (result_tx, result_rx) = oneshot::channel();
    tx.send(DecodeRequest { /* ... */ result_tx }).unwrap();

    // Receive result
    let result = result_rx.await.unwrap();
    assert!(!result.logits.is_empty());

    // Cleanup
    drop(tx);
    handle.await.unwrap();
}
```

---

### Phase 3: Implement SequenceActor (4-6 hours)

**File**: `src/sequence_actor.rs`

**Implementation Order**:
1. Basic structure + constructor
2. `run()` main loop
3. Token sampling logic
4. Stop condition checking
5. Token output to client

**Key Code Sections**:

```rust
impl SequenceActor {
    pub async fn run(mut self) -> Result<(), Error> {
        // Initialize with prompt
        self.process_prompt().await?;

        // Generation loop
        loop {
            // Request next token
            let token = self.generate_next_token().await?;

            // Check stop condition
            if self.should_stop(token)? {
                break;
            }

            // Send to client
            self.send_token(token).await?;
        }

        // Send completion
        self.send_completion().await?;

        Ok(())
    }

    async fn generate_next_token(&mut self) -> Result<i32, Error> {
        // Get next token from cache or request decode
        if let Some(token) = self.state.cache_tokens.pop_front() {
            return Ok(token);
        }

        // Request decode from batch engine
        let (result_tx, result_rx) = oneshot::channel();

        let request = DecodeRequest {
            seq_id: self.id,
            token: self.get_last_token(),
            n_past: self.state.n_past,
            result_tx,
        };

        self.batch_tx.send(request).await?;

        // Wait for result
        let result = result_rx.await?;

        // Sample token
        let token = self.sample_token(&result.logits)?;

        self.state.generated_tokens.push(token);
        self.state.n_past += 1;

        Ok(token)
    }

    fn should_stop(&mut self, token: i32) -> Result<bool, Error> {
        // EOG check
        if self.is_eog(token) {
            return Ok(true);
        }

        // Token-based stop sequences
        if self.stop_matcher.check_token_stop(&self.state.generated_tokens) {
            return Ok(true);
        }

        // String-based stop sequences (if needed)
        if !self.stop_matcher.string_patterns.is_empty() {
            let text = self.detokenize(&self.state.generated_tokens)?;
            if self.stop_matcher.check_string_stop(&text) {
                return Ok(true);
            }
        }

        // Max tokens
        if let Some(max) = self.task.max_tokens {
            if self.state.generated_tokens.len() >= max {
                return Ok(true);
            }
        }

        Ok(false)
    }

    async fn send_token(&self, token: i32) -> Result<(), Error> {
        let text = self.detokenize(&[token])?;

        self.output_tx.send(Ok(TaskResponse::Token(text))).await?;

        Ok(())
    }
}
```

**Test**:
```rust
#[tokio::test]
async fn test_sequence_actor_generation() {
    // Setup channels
    let (batch_tx, batch_rx) = mpsc::unbounded_channel();
    let (output_tx, mut output_rx) = mpsc::channel(100);

    // Create actor
    let task = Task {
        prompt: "Hello".to_string(),
        max_tokens: Some(10),
        // ...
    };

    let actor = SequenceActor::new(0, task, batch_tx, output_tx);

    // Run actor
    tokio::spawn(actor.run());

    // Mock batch engine responses
    tokio::spawn(async move {
        while let Some(req) = batch_rx.recv().await {
            // Send fake logits
            let logits = vec![0.1; 1000];
            let _ = req.result_tx.send(DecodeResult { seq_id: 0, logits });
        }
    });

    // Collect outputs
    let mut tokens = Vec::new();
    while let Some(Ok(TaskResponse::Token(text))) = output_rx.recv().await {
        tokens.push(text);
    }

    assert!(!tokens.is_empty());
}
```

---

### Phase 4: Implement Coordinator (2-3 hours)

**File**: `src/coordinator.rs`

**Implementation Order**:
1. Basic structure + constructor
2. `submit_task()` - spawn actor
3. Actor cleanup logic
4. Integration with existing API

**Key Code Sections**:

```rust
impl Coordinator {
    pub fn new(
        model: Arc<LlamaModel>,
        n_ctx: i32,
        n_batch: i32,
        n_ubatch: i32,
    ) -> Result<Self, Error> {
        // Create channels
        let (batch_tx, batch_rx) = mpsc::unbounded_channel();

        // Spawn batch engine
        let engine = BatchEngine::new(model.clone(), n_ctx, n_ubatch, batch_rx)?;
        tokio::spawn(engine.run());

        Ok(Self {
            batch_tx,
            actors: HashMap::new(),
            next_id: AtomicUsize::new(0),
        })
    }

    pub fn submit_task(&mut self, task: Task) -> Receiver<TaskResponse> {
        let seq_id = self.next_id.fetch_add(1, Ordering::SeqCst);

        let (output_tx, output_rx) = mpsc::channel(100);

        let actor = SequenceActor::new(
            seq_id,
            task,
            self.batch_tx.clone(),
            output_tx,
        );

        let handle = tokio::spawn(async move {
            if let Err(e) = actor.run().await {
                tracing::error!("Actor {} failed: {}", seq_id, e);
            }
        });

        self.actors.insert(seq_id, handle);

        // Cleanup finished actors (async)
        self.cleanup_finished();

        output_rx
    }

    fn cleanup_finished(&mut self) {
        self.actors.retain(|id, handle| {
            if handle.is_finished() {
                tracing::debug!("Cleaning up finished actor {}", id);
                false
            } else {
                true
            }
        });
    }
}
```

**Test**: Integration test with actual model

---

### Phase 5: Integration & Migration (2-3 hours)

**Tasks**:
1. Update `chat_completions.rs` to use Coordinator
2. Feature flag to switch between old/new implementation
3. Side-by-side comparison tests
4. Performance benchmarking

**Migration Path**:

```rust
// In chat_completions.rs
pub async fn chat_completions(
    State(state): State<AppState>,
    Json(request): Json<ChatCompletionRequest>,
) -> Response {
    #[cfg(feature = "actor-model")]
    {
        use crate::coordinator::Coordinator;
        // New implementation
    }
    #[cfg(not(feature = "actor-model"))]
    {
        use crate::parallel::ParallelProcessor;
        // Old implementation (current)
    }
}
```

**Test Both Implementations**:
```bash
# Test old implementation
cargo test

# Test new implementation
cargo test --features actor-model

# Compare performance
./test_chat_completion.sh
./test_chat_completion.sh --actor-model
```

---

### Phase 6: Performance Validation (2-3 hours)

**Benchmarks to Run**:

1. **Latency Test**: Single request time to first token
2. **Throughput Test**: Concurrent requests handling
3. **CPU Usage Test**: Idle and under load
4. **Memory Test**: Memory consumption comparison

**Expected Results**:
- Latency: Within 10% of current (acceptable: <5ms increase)
- Throughput: Same or better (parallel processing)
- CPU: 0.5-1% (acceptable: <0.2% increase from 0.4-0.8%)
- Memory: Slightly higher (actor overhead)

**Performance Script**:
```bash
#!/bin/bash
# performance_comparison.sh

echo "=== Current Implementation ==="
./test_chat_completion.sh
ps aux | grep server | awk '{print "CPU: "$3"%"}'

echo "=== Actor Implementation ==="
cargo run --features actor-model &
sleep 10
./test_chat_completion.sh
ps aux | grep server | awk '{print "CPU: "$3"%"}'
```

---

## Testing Strategy

### Unit Tests

Each component should have comprehensive unit tests:

```rust
// src/sequence_actor.rs
#[cfg(test)]
mod tests {
    #[test]
    fn test_stop_matcher() { /* ... */ }

    #[test]
    fn test_token_sampling() { /* ... */ }

    #[tokio::test]
    async fn test_generation_loop() { /* ... */ }
}

// src/batch_engine.rs
#[cfg(test)]
mod tests {
    #[tokio::test]
    async fn test_batch_collection() { /* ... */ }

    #[tokio::test]
    async fn test_concurrent_requests() { /* ... */ }
}
```

### Integration Tests

Reuse existing `test_chat_completion.sh`:

```bash
# All tests should pass with both implementations
cargo test
cargo test --features actor-model

./test_chat_completion.sh
./test_chat_completion.sh --actor-model
```

### Regression Tests

Create comprehensive test suite:

```rust
// tests/regression.rs
#[tokio::test]
async fn test_all_scenarios() {
    let scenarios = vec![
        ("simple", "Hello", 10),
        ("stop_sequence", "Count to 10", 50),
        ("max_tokens", "Write essay", 100),
        ("concurrent", "Multiple", 20),
    ];

    for (name, prompt, max_tokens) in scenarios {
        test_scenario(name, prompt, max_tokens).await;
    }
}
```

---

## Risk Mitigation

### Risk 1: Performance Regression

**Mitigation**:
- Keep both implementations with feature flag
- Extensive benchmarking before switching
- Gradual rollout (A/B testing)

**Rollback Plan**:
```bash
# If performance is not acceptable
cargo build  # without actor-model feature
# Continue with current implementation
```

### Risk 2: Subtle Behavioral Changes

**Mitigation**:
- Run both implementations in parallel
- Compare outputs token-by-token
- Extensive integration testing

**Detection**:
```rust
// tests/comparison.rs
#[tokio::test]
async fn test_output_equivalence() {
    let prompts = load_test_prompts();

    for prompt in prompts {
        let output_old = run_with_polling(prompt).await;
        let output_new = run_with_actor(prompt).await;

        assert_eq!(output_old, output_new, "Outputs must match");
    }
}
```

### Risk 3: Message Deadlocks

**Mitigation**:
- Use bounded channels where appropriate
- Implement timeouts for all recv() operations
- Comprehensive error handling

**Detection**:
```rust
// Add timeout to all receives
tokio::select! {
    result = rx.recv() => { /* ... */ }
    _ = tokio::time::sleep(Duration::from_secs(30)) => {
        panic!("Deadlock detected!");
    }
}
```

### Risk 4: Increased Memory Usage

**Mitigation**:
- Monitor memory during development
- Use bounded channels (not unbounded)
- Implement actor cleanup

**Monitoring**:
```bash
# memory_test.sh
#!/bin/bash
echo "Monitoring memory usage..."
watch -n 1 'ps aux | grep server | awk "{print \$6}"'
```

---

## Rollout Strategy

### Phase 0: Development (Week 1-2)
- Implement actor model with feature flag
- All tests pass
- Performance validation

### Phase 1: Canary Deployment (Week 3)
- Deploy with actor-model to 10% of traffic
- Monitor metrics closely
- Compare with baseline

### Phase 2: Gradual Rollout (Week 4-6)
- 25% traffic
- 50% traffic
- 75% traffic
- Monitor at each step

### Phase 3: Full Migration (Week 7)
- 100% traffic on actor model
- Remove old implementation after 2 weeks of stability

### Rollback Triggers
- Latency increase >10%
- CPU usage increase >20%
- Any crashes or panics
- Error rate increase >1%

---

## Success Criteria

### Must Have
- ✅ All existing tests pass
- ✅ Performance within 10% of baseline
- ✅ No new bugs for 2 weeks

### Nice to Have
- ✅ Code reduction >30% (target: 762 → 350 lines)
- ✅ Unit test coverage >80%
- ✅ New feature added using actor model

---

## Alternative Approaches Considered

### 1. Hybrid Approach

Keep current loop but extract responsibilities:

```rust
// Half-step refactoring
impl ParallelProcessor {
    async fn run_processing_loop() {
        loop {
            let tasks = self.receive_tasks();
            let ready = self.collect_ready_slots();

            // Extract this
            let sampler = SamplingService::new();
            let stopper = StopConditionChecker::new();

            for slot in ready {
                let token = sampler.sample(slot);
                if stopper.should_stop(slot, token) { /* ... */ }
            }
        }
    }
}
```

**Pros**: Less invasive, easier to implement
**Cons**: Doesn't address core architectural issues

### 2. Stream-Based Architecture

Use Rust streams instead of actors:

```rust
let token_stream = generate_tokens(prompt)
    .take_while(|token| !should_stop(token))
    .map(|token| detokenize(token));

for await token in token_stream {
    send_to_client(token).await;
}
```

**Pros**: More idiomatic Rust, functional style
**Cons**: Still needs batch engine, complex backpressure

### 3. Do Nothing

Keep current implementation:

**Pros**: No risk, works today, well-tested
**Cons**: Technical debt accumulates, harder to maintain over time

---

## References

### Discussions
- Original refactoring TODO: `/opt/hippocampus/cluster/applications/rust_de_llama/TODO.md`
- Architecture discussion: (this document's conversation history)

### Similar Implementations
- vLLM: Python-based continuous batching (reference architecture)
- TGI (Text Generation Inference): Rust implementation
- llama.cpp server: C++ reference implementation

### Rust Patterns
- Tokio actors: https://tokio.rs/tokio/tutorial/channels
- Message passing: https://doc.rust-lang.org/book/ch16-02-message-passing.html
- Actor model in Rust: https://ryhl.io/blog/actors-with-tokio/

---

## Appendix A: Message Flow Diagram

```
Time →

Client          Coordinator       SequenceActor       BatchEngine       llama.cpp
  │                 │                    │                  │                │
  ├─submit_task────>│                    │                  │                │
  │                 ├─spawn─────────────>│                  │                │
  │                 │                    ├─DecodeRequest───>│                │
  │                 │                    │                  ├─collect (1ms)─>│
  │                 │                    │                  │                │
  │                 │                    │         ┌────────┴────────┐       │
  │                 │                    │         │ More requests   │       │
  │                 │                    │         │ from other      │       │
  │                 │                    │         │ actors...       │       │
  │                 │                    │         └────────┬────────┘       │
  │                 │                    │                  │                │
  │                 │                    │                  ├─build_batch───>│
  │                 │                    │                  │                │
  │                 │                    │                  │<──decode───────┤
  │                 │                    │                  │                │
  │                 │                    │<──DecodeResult───┤                │
  │                 │                    ├─sample_token────>│                │
  │                 │                    ├─check_stop──────>│                │
  │<────Token───────┴────────────────────┤                  │                │
  │                 │                    │                  │                │
  │                 │                    ├─DecodeRequest───>│                │
  │                 │                    │    (next token)  │                │
  │                 │                    │                  │                │
  ...              ...                  ...                ...              ...
```

---

## Appendix B: Performance Benchmarks

### Baseline (Current Implementation)

```
Model: gemma-3-4b-it-Q4_K_M.gguf
Hardware: [Your specs]

Single Request:
- Time to first token: 150ms
- Tokens per second: 45 t/s
- CPU usage (idle): 0.6-0.8%
- CPU usage (generating): 0.4%

Concurrent Requests (n_parallel=2):
- Both complete: 3.5s
- Total tokens: 40
- CPU usage: 0.4%
```

### Target (Actor Model)

```
Single Request:
- Time to first token: <165ms (+10%)
- Tokens per second: >40 t/s (-10% acceptable)
- CPU usage (idle): 0.7-1.0%
- CPU usage (generating): <0.6%

Concurrent Requests (n_parallel=2):
- Both complete: <3.9s (+10%)
- Total tokens: 40
- CPU usage: <0.6%
```

---

## Appendix C: Code Size Comparison

### Current Implementation
```
src/parallel.rs: 762 lines
├─ Imports & Types: 50 lines
├─ Slot: 80 lines
├─ ActiveSequence: 40 lines
├─ StopMatcher: 80 lines
├─ BatchSlotInfo: 4 lines (struct)
├─ BatchItem: 5 lines (struct)
├─ ParallelProcessor: 503 lines
│  ├─ new(): 30 lines
│  ├─ submit_task(): 10 lines
│  ├─ run_processing_loop(): 80 lines
│  ├─ process_batch(): 30 lines
│  ├─ decode_prompts(): 25 lines
│  ├─ generate_tokens(): 70 lines
│  ├─ check_stop_sequences(): 80 lines
│  ├─ tokenize/detokenize: 20 lines
│  └─ helpers: 158 lines

Total: 762 lines
```

### Actor Model (Estimated)
```
src/sequence_actor.rs: 150 lines
├─ SequenceActor struct: 20 lines
├─ run() main loop: 20 lines
├─ generate_next_token(): 30 lines
├─ should_stop(): 25 lines
├─ send_token(): 10 lines
├─ helpers: 45 lines

src/batch_engine.rs: 100 lines
├─ BatchEngine struct: 15 lines
├─ run() main loop: 20 lines
├─ collect_requests(): 25 lines
├─ build_batch(): 20 lines
├─ send_results(): 20 lines

src/coordinator.rs: 50 lines
├─ Coordinator struct: 10 lines
├─ new(): 15 lines
├─ submit_task(): 20 lines
├─ cleanup: 5 lines

src/actor_types.rs: 50 lines
├─ DecodeRequest: 10 lines
├─ DecodeResult: 5 lines
├─ GenerationState: 10 lines
├─ Shared types: 25 lines

Total: ~350 lines (-54%)
```

---

## Conclusion

This refactoring represents a significant architectural improvement that prioritizes long-term maintainability and extensibility over short-term convenience. While the current implementation works well, the actor model provides a more sustainable foundation for future development.

**Decision**: Implement when maintenance pain becomes evident or new features require it.

**Next Steps**:
1. Review this document with team
2. Decide on timeline based on current priorities
3. Begin with Phase 0 (parallel implementation)
4. Proceed incrementally with continuous validation

---

**Document Version**: 1.0
**Last Updated**: 2025-10-30
**Authors**: Based on architectural discussion and code analysis
**Status**: Ready for Implementation
