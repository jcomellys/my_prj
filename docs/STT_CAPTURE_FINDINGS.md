# STT capture hardening notes

Date: 2026-05-19

## Context

The first controlled A/B run showed that the main user-facing risk was not only
the Whisper model size. Several clips lost the first command word or were so
short that Whisper produced hallucinated text. That is dangerous for an
accessibility agent because the brain can only behave safely if the transcript
is trustworthy.

## Evidence

Baseline on `samples_ab2/manifest.txt`:

- `stt_ab2.log`: `small` 0/10 exact matches, 0.60s average latency.
- `stt_ab2.log`: `medium` 3/10 exact matches, 1.55s average latency.
- Qualitative failure: leading verbs such as "abre", "cierra", "describe" were
  often missing. The 0.25s "qué hora es" clip was not reliable audio.

Padding-only experiment:

- `stt_ab2_pad500.log`: adding 0.50s leading silence and 0.20s trailing silence
  improved `small` to 2/10 and `medium` to 4/10.
- This proved that Whisper benefits from a little context around short command
  clips, but padding alone is not enough.

Tuned pipeline experiment:

- `stt_ab2_tuned.log`: with 0.50s leading pad, 0.20s trailing pad, 0.30s minimum
  duration, and a concrete Spanish command prompt containing the target command
  forms, `small` reached 7/10 and `medium` reached 8/10.
- The 0.25s "qué hora es" clip was skipped instead of being sent to Whisper and
  hallucinated.
- `medium` still missed the YouTube/Beethoven phrase, so the model-promotion
  decision is not fully closed.

Vocabulary-prompt experiment:

- `stt_ab2_tuned_vocab_prompt.log`: replacing full target phrases with a more
  honest vocabulary prompt reached `small` 2/10 and `medium` 6/10.
- This is less impressive than the full-command prompt, but it is the safer
  product default because it does not look like the evaluation set was embedded
  into the runtime config.

Clean re-recorded corpus:

- `samples_clean_01/manifest.txt` was recorded with the countdown flow. Two
  short clips were flagged (`abre Mensajes` at 0.75s and `qué hora es` at
  0.77s), but both were still intelligible in the tuned run.
- `stt_clean_01_raw_normfix.log`: without padding/prompt/min-duration tuning,
  `small` reached 5/10 and `medium` reached 9/10.
- `stt_clean_01_tuned_vocab_normfix.log`: with the tuned pipeline and the
  vocabulary prompt, `small` reached 7/10 and `medium` reached 10/10.
- The tuned `medium` run correctly handled the important cases that had failed
  earlier: `abre Mensajes`, `cierra esta pestaña`, `qué hora es`, Beethoven, and
  the Word/title command.

Negative control:

- `stt_ab2_tuned_general_prompt.log`: a generic command prompt underperformed
  badly (`small` 1/10, `medium` 4/10).
- Conclusion: if we use a Whisper prompt, it should contain concrete command
  examples, not abstract product language.

## Local changes made for review

- `internal/stt/whisper.go`
  - Rejects clips shorter than `MinDurationSeconds` before Whisper runs.
  - Pads recorded audio before transcription with configurable leading/trailing
    silence.
  - Supports an optional Whisper initial prompt and optional `-nth` override.

- `internal/agent/config.go` and `cmd/agent/main.go`
  - Wires the new Whisper tuning fields through YAML config.
  - Numeric tuning fields preserve the difference between "omitted" and an
    explicit `0`, so operators can disable padding/min-duration behavior if a
    future profile needs that.

- `config.example.yaml`
  - Adds tuned defaults for `voice` and `manos_libres`: minimum duration,
    leading/trailing padding, and a Spanish vocabulary prompt.

- `scripts/record_phrases.sh`
  - Adds a countdown and duration warning so future corpora are less likely to
    be contaminated by the operator speaking before the mic is ready.

- `scripts/whisper_ab.sh`
  - Keeps the original A/B behavior by default.
  - Adds optional env vars so capture/prompt tuning can be measured without
    editing the script:
    - `LEADING_PAD_SECONDS`
    - `TRAILING_PAD_SECONDS`
    - `MIN_DURATION_SECONDS`
    - `WHISPER_PROMPT`
    - `NO_SPEECH_THRESHOLD`
  - Normalizes accents before exact-match scoring, matching the documented
    metric behavior.

## Verification

- `bash -n scripts/whisper_ab.sh scripts/record_phrases.sh` passed.
- `go test ./internal/stt ./internal/agent ./cmd/agent` passed.
- `go test ./...` passed.
- `go test -race -count=1 ./...` passed.
- For the recorded WAV corpus, the internal byte-size duration estimate matched
  `soxi -D` for all 10 files.
- Clean corpus result after accent-normalization fix:
  - raw: `small` 5/10, `medium` 9/10.
  - tuned vocabulary pipeline: `small` 7/10, `medium` 10/10.

## Recommendation

Do not jump to cloud STT yet. The local path still has room because simple
capture conditioning raised the controlled score substantially.

The clean corpus supports promoting `medium` for the `manos_libres` profile when
paired with capture conditioning. The evidence currently says:

- `medium` is better on names and multi-word commands.
- Latency is acceptable on the M4.
- On the clean corpus, tuned `medium` had no exact-match regressions.
- Prompting can help, but full target-command prompts should be treated as an
  upper-bound experiment, not as proof of production quality.

Next best gate: run a short live `manos_libres` smoke test using `medium` plus
the tuned capture defaults. If the live loop matches the corpus result, promote
`medium` for hands-free voice and keep `small` only for a fast/low-resource
profile.
