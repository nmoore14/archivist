# Model evaluation

This evaluation compares the same local chat model under two conditions:

- `no_rag`: The model receives only the question.
- `rag`: The model receives the question and the top three chunks retrieved from
  the bundled course text.

The runner fixes the generation temperature at `0` and the seed at `42`. It
records the dataset and source hashes, model names, retrieval scores, generated
answers, latency, and runtime environment for each run.

## Before you begin

Start Ollama and download the default models:

```sh
ollama pull gemma3:1b
ollama pull nomic-embed-text
```

If Ollama runs through the repository's Docker Compose configuration, run the
evaluation inside the application container or set `OLLAMA_BASE_URL` to an
address that the host can reach.

## Validate the evaluation files

Run the offline validation and regression tests:

```sh
make evaluate-test
```

This command doesn't require Ollama.

## Run the A/B evaluation

```sh
make evaluate
```

Each run creates a timestamped directory under `results/evaluation/` containing:

- `run.json`: Complete machine-readable configuration and results.
- `results.csv`: Flat results for analysis or charting.
- `summary.md`: Automated screening metrics and latency.
- `manual-review.csv`: Blank correctness, groundedness, and notes columns for a
  human reviewer.

The command returns a nonzero exit status if any model request fails. It keeps
the artifacts for debugging, but you must not interpret failed requests as
incorrect model answers.

To use an explicit output directory or model:

```sh
python3 scripts/evaluate.py \
  --chat-model gemma3:1b \
  --output results/evaluation/report-run
```

The output directory must not already exist. This prevents a new run from
overwriting earlier evidence.

## Review and report the results

The automated score measures coverage of documented concepts and whether an
unanswerable question avoids known hallucinated details. Treat it as a
reproducible screening check, not a semantic correctness score.

For the assignment report, complete `manual-review.csv` with this rubric:

- Correctness `2`: Correct and complete.
- Correctness `1`: Partially correct or incomplete.
- Correctness `0`: Incorrect, unsupported, or missing.
- Groundedness `1`: Every factual claim is supported by the retrieved context.
- Groundedness `0.5`: Mostly supported, with a minor unsupported claim.
- Groundedness `0`: Material claims are unsupported.

Classify recurring failures using the generated `error_category` field, and add
more specific notes when failures result from chunk boundaries, weak retrieval,
unsupported generation, incorrect abstention, or an operational error.

## Change the dataset

The dataset is versioned in `cases.json`. When you change questions, expected
concepts, or scoring rules, update `dataset_version`. Keep evaluation questions
independent from prompts used to tune the application.
