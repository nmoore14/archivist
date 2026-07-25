# Archivist Baseline Model

This repository contains a reproducible Jupyter experiment for a lightweight, local Retrieval-Augmented Generation (RAG) pipeline. It compares the same 15 educational questions under two conditions: a small Ollama chat model without context and the same model with retrieved context from OpenStax *Principles of Data Science*, section 1.3.

The notebook records model responses, retrieval results, latency, and approximate Python-process memory. Accuracy and groundedness remain blank until a person reviews the answers; the project does not fabricate evaluation results.

## Required software

- Python 3.10 or newer
- [uv](https://docs.astral.sh/uv/)
- [Ollama](https://ollama.com/) running locally
- JupyterLab (installed into the uv environment)

## Setup

Create or update the environment from `pyproject.toml`:

```bash
uv sync
```

Equivalent dependency command for a new project:

```bash
uv add ollama pandas numpy scikit-learn matplotlib psutil jupyter
```

Download and start the local models:

```bash
ollama pull qwen3:0.6b
ollama pull nomic-embed-text
```

Ollama should be available at `http://localhost:11434`. Model names and the host can be changed together in the notebook configuration cell.

Launch the notebook:

```bash
uv run jupyter lab
```

Open `baseline_archivist.ipynb` and run cells from top to bottom. The costly experiment is opt-in: set `RUN_EXPERIMENT = True` in the experiment cell. `FORCE_RERUN = False` resumes completed rows from `results/baseline_results.csv`; set it to `True` only when answers should be regenerated.

## Directory structure

```text
.
├── baseline_archivist.ipynb
├── baseline_report_template.md
├── data
│   ├── openstax_questions.csv
│   └── openstax_section_1_3.txt
├── results
│   └── .gitkeep
├── pyproject.toml
└── README.md
```

The bundled section text is an attributed, instructor-prepared paraphrase that supports offline execution. Review it against the assigned OpenStax section, or replace it with an instructor-approved local text export before final submission. The notebook also contains a clearly labeled fallback sample if the file is missing.

## Manual scoring workflow

1. Run the experiment for all 15 questions.
2. Open `results/baseline_results.csv` in a spreadsheet editor, or copy the notebook review table.
3. Enter only allowed rubric values:
   - correctness: `0`, `1`, or `2` for both conditions;
   - RAG groundedness: `0`, `0.5`, or `1`;
   - retrieval hit: `0` or `1` for answerable questions;
   - hallucination handling: `0` or `1` for the two unanswerable questions.
4. Add review notes where useful, save the CSV, then rerun the notebook’s manual-review and metrics cells.
5. Use `results/report_summary.txt` and `baseline_report_template.md` to prepare the 1–2 page report.

The metrics cell validates scores and refuses to summarize invalid or incomplete required fields. Scores are human judgments, not automated string-match claims.

## Export

After running and saving the notebook, export it from JupyterLab or use:

```bash
uv run jupyter nbconvert --to html baseline_archivist.ipynb
uv run jupyter nbconvert --to webpdf baseline_archivist.ipynb
```

The PDF route may require an additional browser dependency. HTML export is the more portable option.

## Known limitations

- Fifteen questions from one textbook section form a small, non-independent evaluation set.
- Manual scoring is subjective unless multiple reviewers score independently.
- The paraphrased source may differ from a full instructor-approved export.
- Character-based chunking is not tokenizer-aware.
- Retrieval uses dense embeddings and cosine similarity without reranking or metadata filters.
- The recommended 0.6B chat model prioritizes low resource use over answer quality.
- Python RSS does not include all memory used by the separate Ollama process or model runtime.
- Latency depends on local hardware, model state, and whether models are already loaded.
