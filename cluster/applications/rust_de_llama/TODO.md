# TODO

Work items on the Thinker-Talker architecture in this application and `notebooks/`.
The goal these were identified against — chat with any open-weight LLM and hear its reply as speech — is met: `/v1/chat/completions` with `modalities: ["text", "audio"]` speaks a chat model's reply through a pretrained talker and the WavTokenizer decoder.
What remains is the half of the architecture that carries the thinker's hidden states rather than its text, and the conversational depth that needs.
Voice selection is out of scope for now (planned as a voice-changer style post-processor), and so is speaking anything but English (the talker's card declares `zh, ja, ko`, but every route to them runs through the WavTokenizer encoder this repository does not ship).
Items are ordered by impact within each category.

The serving layer is tracked separately under [Scheduling](#scheduling): it is the same binary but a different axis, and its goal — beating `llama-server` at the configuration this repository ships — is not met.

## Speech quality

1. **Train the projection.** *(the data problem is solved and the wiring is
   proven; the objective cannot tell a projection that encodes text from one
   that memorized the answer, and every run so far has been the second. That is
   what is left.)*

   The projection is the remaining half of the architecture: it carries the
   thinker's hidden states instead of its text, which is what the
   `<|text_sep|>`-normalized talker-native path cannot do — that path throws
   away everything but letters, so prosody has nothing to condition on and only
   English survives at all. (Self-distillation cannot deliver that motivation;
   see the ceiling below.)

   **The wiring is proven and the objective is not.** Four runs, all judged by
   free running (the notebook's diagnostic cell drives the talker from the
   projection alone, no teacher forcing, and prints the words it says) on
   *training* sentences:

   | corpus | exposures | loss | what it said, given "Spot saw the shiny car and said, Wow..." |
   |--------|-----------|------|--------------------------------------------------------------|
   | (none) | 0 | 2.66 | `robert meyer is currently working on a bachelors program` — the talker's own prior, ignoring the projection |
   | 8 | 100 | **0.25** | `spot saw the shiny car and said wow kitty your car is so bright and clean` — **every word, stops on its own** |
   | 1200 | 6 | 1.22 | `lily thought that the biggest thing needed to make it to a skypole` — fluent TinyStories-ish, i.e. the corpus's marginal, not its conditional |
   | 150 | 40 | 0.68 | `spot saw the shiny car and said wow` then loses the thread |
   | 150 | 80 | **0.39** | `spot saw the` then `spot the next minute and shiny colors like pins thank size seven ohm` — **worse at half the loss** |

   The 8-sample run settles that the architecture, the prompt assembly, the
   embedding hand-off and the stop condition are all correct: the projection can
   carry a sentence into the talker. It settles nothing else — see below.

   **Every trained run in that table scores below what the correct text does.**
   Feed the talker the real normalized sentence — the verified talker-native
   path, and so the best any encoding of the sentence could possibly do — and
   score the same recorded continuation against it:

   | condition | non-code CE | code CE | total |
   |-----------|-------------|---------|-------|
   | random projection (the floor) | 1.327 | 3.026 | **2.66** |
   | wrong sentence's text, right prefix | 1.375 | 3.026 | 2.67 |
   | **the true text (the ceiling)** | **0.760** | **2.997** | **~2.51** |

   The floor reproduces the table's untrained 2.66 exactly, which is what says
   the measurement is faithful. So the entire span the objective can express —
   carrying nothing, to carrying the sentence perfectly — is **0.15 nats**, and
   all four runs (1.22, 0.68, 0.39, 0.25) sit far below the bottom of it.

   Nothing that encodes the sentence can score there. Codes are 78.4% of the
   supervised positions, so a total loss of *L* holds code CE to at most
   *L*/0.784: the 8x100 run's 0.25 puts its code CE at **0.32 at most**, against
   the 3.0 the *correct text* gets. And code CE ~3 is a floor rather than a
   failure to learn — the codes are irreducibly flat, with the talker re-reading
   its own greedy output still putting only p~0.08 on the code it picked itself,
   about 12 effective choices out of 4096.

   **So the projection is not encoding text; it is memorizing the code
   sequence.** Its output is ~13x896 continuous values per sentence, while
   selecting one of 150 recorded continuations takes 7.2 bits — memorizing is by
   far the cheapest way down. That reading fits every row: 8x100 memorizes 8
   continuations and is then tested on those same 8; 1200x6 cannot memorize 1200
   and falls back to the corpus's marginal; 150x40 -> 150x80 memorizes harder
   and free running rots. **The loss falling below ~2.3 is the symptom, not the
   progress.**

   **This retires both fixes this item used to propose.** Both rested on the
   claim that ~29 of every 30 supervised positions are audio codes that "own the
   gradient". Measured, both halves are wrong:

   - The ratio is **78.4% codes / 21.6% words+markers+stop** (word pieces 9.6%,
     durations 3.9%, `<|code_start|>` 3.9%, `<|code_end|>` 3.9%, stop 0.4%) —
     one non-code position in 4.6, not in 30.
   - The codes own the *loss value* (87.6% of it) but **not the gradient**.
     Splitting one batch's backward pass by category: codes deposit a gradient
     of norm **0.50** on the projection, words+markers+stop deposit **14.03**.
     The chain rule already de-weights the codes 28x, for exactly the reason the
     old text gave for masking them — a code that follows from a word already in
     context has almost no derivative with respect to the projection.

   **Masking the code positions is therefore a 4.37x learning-rate increase
   wearing a costume**: cosine(current gradient, code-masked gradient) =
   **0.9927**. It cannot fix the regression; it arrives there sooner. And code
   CE is the one place memorization is visible in the loss, so masking it out
   would leave the loss looking clean while the same thing happens underneath.

   **Early stopping on the diagnostic** fails the same way. The diagnostic free-
   runs on *training* sentences, which is precisely what memorization aces, so
   stopping on it selects for memorization. The old text called this "the easier
   of the two questions"; it is not a weaker form of the right question but a
   different one, and it cannot see the failure that is actually occurring.

   **What is actually left:**

   - **The diagnostic has to be held-out.** It is the only gauge this work
     trusts, and on training sentences it cannot separate encoding from
     memorization. Nothing else here is measurable until this changes. It is
     also the cheapest test of the memorization reading: reproduce 8x100 (~400
     steps, ~20 min) and free-run it on a sentence it never saw.
   - **The loss has a usable range, and it is not "lower is better".** ~2.66 is
     carrying nothing, ~2.51 is carrying the sentence perfectly. A run
     converging toward ~2.5 whose held-out free running works is the target; a
     run heading for 0.3 has left the text manifold.
   - **The channel is over-provisioned and nothing constrains it.** 3.0M
     parameters and 11,648 free values per sentence, against 7.2 bits of "which
     continuation". Tying the projection to the text — an auxiliary loss against
     the talker's own text embeddings for the normalized sentence, or simply far
     less capacity — is what the measurements point at. This is a design
     question, not a hyperparameter.

   **A ceiling worth naming before spending more here.** The targets are the
   talker's own greedy output given the *normalized* text, so they are a
   deterministic function of it: the recorded codes carry exactly zero
   information about the case and punctuation that survive only in the raw
   sentence. Self-distillation therefore cannot teach the projection to use
   them — the training signal for prosody is not in the targets. Its best
   possible outcome is to *match* talker-native, not to beat it. That is still
   the milestone that proves the hidden-state path end to end, and worth having.
   The same argument retires the other half of the motivation: self-distillation
   takes talker-native as its teacher, so it cannot bootstrap any language that
   talker-native cannot already say. Carrying what normalization throws away —
   prosody or otherwise — needs real recordings, the same gap item 2 hits.

   **The targets no longer need recordings.** `notebooks/thinker-talker-projection.ipynb`
   distills them out of the talker itself: the talker reads each corpus
   sentence with its own tokenizer — the verified talker-native path — and the
   continuation it generates becomes the supervision for the same talker driven
   from the projected hidden states instead. The targets are therefore already
   in the speaker prompt's voice and the talker's own format, and no
   WavTokenizer encoder is involved. This is what replaced the old ask for
   `data/train.json` full of real text-audio pairs; the alternative (LJSpeech
   plus a WavTokenizer encoder) fights the speaker conditioning, since the
   talker is prompted with a specific reference voice, and would need forced
   alignment to rebuild OuteTTS's per-word timing structure.

   Three defects in the old notebook had to be fixed before any amount of data
   would have helped:
   - The text was padded to `max_length` and **every padded position was
     projected and fed to the talker**, putting dead embeddings exactly where it
     expects words. Batches are now bucketed by thinker-token length instead.
   - The stop token was not supervised, so the projected path could never learn
     to end and would run to `MAX_CODES` every time. The recorded continuation
     now includes it.
   - The thinker read the same normalized text the talker did, which throws away
     the case and punctuation the projection exists to carry. It now reads the
     raw sentence — though per the ceiling above, self-distilled targets cannot
     reward it for using them; this is the architecture being right ahead of the
     data, not a gain available today.

   Measured on this host (Ryzen 9 7950X, 16 cores, no GPU, ~15 GB free):
   distillation runs at ~4 s per utterance and training at ~3 s per step, so
   1200 sentences over 6 epochs is ~4 hours. bf16 (avx512_bf16) is what makes
   this practical — the first measurement, at fp32 with 32 threads on 16
   physical cores, was 20x slower, which was a measurement error rather than a
   property of the machine. **Compute is not the blocker it was assumed to be**,
   and neither is the corpus/exposures trade-off: what blocks this is that the
   objective cannot rank projections, so no schedule over it finds a good one.

2. **Distillation cannot teach what the talker could not already say.** The
   corpus is digit-free English, so numbers are outside the trained
   distribution. The projected path does not normalize at all, so digits reach
   the thinker raw. Getting past this needs targets from real recordings, which
   is the data problem item 1 sidestepped rather than solved.

## Conversation

3. **Streaming synthesis.** Generation is one-shot (all text -> all codes -> one
   iSTFT), so latency grows linearly with length and `/v1/chat/completions`
   refuses `stream` together with `audio`. A conversational flow wants chunked
   or streaming synthesis end to end; the talker already emits codes one at a
   time, so the AR loop is the natural place to start. Reaching the deeper
   Thinker-Talker form — speaking from the hidden states of the reply's own
   generating forward pass, rather than re-reading the finished text — depends
   on this and on item 1.

## Generalization to any open-weight LLM

4. **Per-model training recipe.** The projection must be trained per thinker
   (`input_dim` = that model's `n_embd`, and the hidden-state distribution is
   model-specific). The notebook is parameterized by `THINKER_MODEL` and
   checkpoints reject a resume across dimensions, but the path from a HF model
   name to a projection GGUF registered in `models.toml` is not scripted end to
   end. Note that Jupyter kernels cannot start in the sandbox (ZeroMQ cannot
   bind a local socket), so whatever drives the notebook headlessly cannot be
   nbconvert.
5. **The thinker's quantization shifts the hidden states, and it is measured.**
   `test_thinker_hidden_states_match_the_full_precision_reference` checks the
   states the runtime reads from a quantized GGUF against the fp32 HF ones the
   projection trains on (a fixture the notebook's section 14 writes). The
   tokenizers agree exactly (neither adds BOS), so positions align; the
   magnitudes drift by **8.6%** worst-case at Q8_0 and **25.8%** at Q4_K_M,
   growing along the sentence as error accumulates through the causal context.
   Cosine stays high (0.966 even at Q4_K_M), so the direction survives and the
   projection will not simply break — but 25.8% is not a rounding difference.

   `models.toml` suggests Q8_0 for `thinker_model` and now has a number behind
   that; Q4_K_M is what the *chat* models run at, so a thinker shared between the
   two paths is where the wrong one gets picked by accident. What is left is that
   none of this is measured against **speech**, only against the states — cheap
   to do once item 1 has a working projection: run the projected path at each
   quantization and transcribe.

## Audio robustness

6. **The speaker scaffolding is re-prefilled per chunk — but almost none of it
   can be cached.** The original item claimed ~900 constant tokens are decoded
   again per run and that keeping their KV would pay for itself. Measured, the
   prompt is `[prefix 70 | text | suffix 818]`: the 818-token speaker *audio*
   sits **after** the variable text, so its KV depends on that text and cannot
   be cached at all. Only the 70-token prefix can — 7.9% of the scaffolding, not
   ~900 tokens. `LlamaContext::remove_sequence_from` already exists to do it;
   whether 70 positions of prefill is worth the complexity is the open question.

## Scheduling

Measured against `llama-server` built from the vendored `llama.cpp` with matching flags (`-c 4096 -np 4 -b 512 -ub 512 -ctk q8_0 -t 16 -ngl 99`), on gemma-3-4b-it-Q4_K_M over CUDA.
At four concurrent requests the two are level: decode 49.4 vs 49.8 tok/s, TTFT 40.8 vs 42.2 ms.
Single-stream decode is ~2% ahead.
Nothing here is a win yet, and the host runs a VM that consumes roughly half the physical cores, so every absolute number is a contended figure that a dedicated machine should re-establish.
Those figures also predate retained-cell expiry, and `llama-server` has not been re-measured since.

1. **Preemption.** *(the one item that makes this scheduler different from
   `llama-server`'s, and the reason *Prompt cache across slots* and *Reserve
   draft cells* exist)*

   Admission currently reserves `prompt + max_tokens` cells before a request
   starts, and defers whatever does not fit. That is pessimistic in both
   directions: a client sending `max_tokens: 4096` reserves the whole pool and
   serialises the server, and the queue head blocks everything behind it. vLLM
   does not reserve — it admits, and evicts a running sequence when the cache
   runs short.

   Both mechanisms exist here. `clear_sequence` already wraps
   `llama_memory_seq_rm`, and `llama.h:825-881` exposes
   `llama_state_seq_get_data` / `set_data`, so a preempted sequence can either
   be recomputed from `prompt_tokens + generated_tokens` (with `max_tokens`
   reduced by what was already streamed, since the client has those bytes) or
   have its KV copied out and restored verbatim. Measure
   `llama_state_seq_get_size` before choosing: swap avoids the re-prefill but
   pays transfer, recompute pays prefill but needs no buffer.

   `Slot::stop_task` drops the `Task`, so preemption needs a sibling that
   returns it for requeueing at the *front* of `pending_tasks` — the back
   starves it.

2. **Prompt cache across slots.** Reuse is currently per-slot: `select_slot`
   scans idle slots' retained tokens, so a prefix survives only while the slot
   that produced it stays idle and at most `RETAINED_PREFIX_DECODE_BUDGET`
   iterations have been batched past it, or
   `RETAINED_PREFIX_CREDITED_DECODE_BUDGET` while the queue head would reuse its
   prefix. `llama_memory_seq_cp` (`llama.h:713`)
   copies KV between sequences, which is what a hash-indexed cache shared by all
   slots would need. This is Proposal 2 of the deleted `IMPROVEMENT.md`, the only
   one of its four never implemented. `llama-server` covers the same ground with
   `--cache-reuse` and an 8 GiB RAM-backed LRU (`-cram`).

3. **Guard prompt-cache reuse against sliding-window eviction.** `select_slot`
   treats `Slot::cached_tokens` as an exact description of what is resident and
   `assign_task_to_slot` prefills only the divergent suffix, but llama.cpp drops
   a *leading* range of an idle sequence without telling anyone:
   `llama-kv-cache.cpp:964` runs `seq_rm(s, cells.seq_pos_min(s),
   seq_pos_max_rm[s] + 1)` when a new slot needs cells the sliding window has
   already masked, and `llama.h` guarantees only `[pos_min, pos_max]` present.
   gemma-3 is affected: `llama-model.cpp:1343-1348` sets
   `LLAMA_SWA_TYPE_STANDARD` with `set_swa_pattern(6)`, so 29 of its 34 layers
   slide. Reuse spanning the hole then attends over missing keys and emits wrong
   tokens with no error raised. Because the missing range is a *leading* one,
   shortening `common_len` cannot repair it: bind `llama_memory_seq_pos_min`
   (absent from `src/lib.rs`) and discard the whole retained prefix when
   `pos_min > max(0, n_past - n_swa)` -- the clamp matters, since without it
   every prefix shorter than `n_swa` trips the test -- which is what
   `llama-server` does at
   `tools/server/server-context.cpp:2277-2331`. A prefix shorter than `n_swa`
   is immune, which is what bounds the exposure. Note `src/wrapper.h` leaves
   `swa_full` at its default of true, so a smaller SWA cache is not an
   alternative mitigation — the steal comes from `find_slot` reusing masked
   cells, which ignores cache size.

4. **Reserve draft cells.** `cells_for` counts `prompt + max_tokens`, but
   speculative decoding writes up to `max_draft` extra cells into the target's
   KV before the rejected tail is rolled back a decode later. The per-sequence
   guard at `remaining_ctx = n_ctx_seq - (position + 1)` stopped bounding the
   shared pool once `kv_unified = true` made `n_ctx_seq == n_ctx`. Four requests
   reserving 1024 each against a 4096-cell pool leave no room for drafts, and a
   failed decode now takes every sequence in the batch down with it. Harmless
   while `[speculation]` stays commented out in `models/models.toml`; fix before
   enabling it.

5. **Narrow `fail_active_slots`.** It ends every non-idle slot, but
   `collect_batch_slots` drops prompt slots once `n_batch` is exhausted. Those
   never entered the failed decode and hold no stale `logits_index`, yet they
   die with it. Pass the batch's slot indices and fail only those.

6. **Reject over-long requests with 400.** `prompt + max_tokens` exceeding the
   context is clamped silently by `cells_for`, so such a request is admitted and
   truncated rather than refused. The prompt-only check in
   `handler/chat_completions.rs` already returns 400 for its half of the same
   condition.

7. **Drop disconnected tasks at the head of the queue.** `pending_tasks` is
   never checked for `response_tx.is_closed()`; only slots are, and only after
   admission. A client that hung up still blocks everything behind it for as
   long as the queue is backed up.

8. **Instrument admission.** Deferrals are invisible, and neither cache purge —
   admission-pressure or budget expiry — has a counter, only a `debug!` line.
   `Metrics` records prefill, decode, batch size, slot occupancy and speculation
   acceptance, so a queue that has stopped moving is indistinguishable from a
   model that is merely slow, and a prompt-cache miss from a purge.

9. **Pin the liveness invariant.** Nothing rejects a request that cannot fit, on
   the grounds that `cells_for` clamps `needed` to `n_ctx_seq` and an empty pool
   therefore always admits. That is true today and load-bearing; a
   `debug_assert!` at the deferral branch would stop a future change to the
   clamp from turning it into a silent hang.

10. **The CUDA build aborts on every graceful shutdown.** SIGTERM reaches the
    lameduck path, the loop tears down, and then `ggml_cuda_error`
    (`ggml/src/ggml-cuda/ggml-cuda.cu:97`) calls `GGML_ABORT`, so the process
    dies of SIGABRT instead of exiting 0. Reproduced 8 times out of 8 with no
    request ever served, which also rules out anything in the serving path;
    `coredumpctl` holds matching dumps going back to 2026-07-21. Under
    Kubernetes every rollout and eviction therefore reports a crashed container,
    and `--lameduck` buys nothing. `GGML_LOG_ERROR` carries the failing
    statement but is not routed anywhere, so installing a `ggml_log_callback`
    is the first step to naming it.
