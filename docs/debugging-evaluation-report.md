# Archivist model testing and debugging report

## Abstract

This evaluation tested whether retrieval-augmented generation (RAG) improves
Archivist's answers over direct local generation. Both conditions used the same
`gemma3:1b` chat model, temperature of `0`, seed of `42`, and 15-question test
set. The no-RAG condition received only each question; the RAG condition also
received the three most similar chunks from a local OpenStax course text,
selected with `nomic-embed-text` embeddings and cosine similarity. RAG increased
the automated pass rate from 6.7% to 80.0% and mean expected-concept coverage
from 0.35 to 0.88. A separate rubric-based review estimated correctness at 56.7%
of the available points without RAG and 90.0% with RAG. RAG added modest latency:
median response time increased from 0.51 to 0.57 seconds. Error analysis found
one retrieval miss, two incomplete RAG answers, unsupported additions in direct
answers, and an earlier operational failure that produced blank answers. The
results support RAG for source-specific educational questions, while also
showing that retrieval success and generation completeness require separate
tests.

## Methodology

The experiment used an A/B design rather than cross-validation because the goal
was to isolate the effect of retrieved context. The versioned dataset,
`openstax-1.3-v1`, contains 15 researcher-written questions about a bundled
paraphrase of OpenStax _Principles of Data Science_, section 1.3. Categories
include definitions, comparisons, examples, reasoning, recall, synthesis, and
two deliberately unanswerable questions. Each question has a reference answer,
expected concepts, and, for unanswerable cases, prohibited details associated
with hallucination.

The runner divided the 6,871-character source into 1,000-character chunks with
200-character overlap. For RAG, it embedded each question and chunk, ranked
chunks by cosine similarity, and supplied the top three to the chat model. For
no-RAG, it supplied only the question. All 30 responses were produced on macOS
ARM64 using Python 3.14.6. The run recorded hashes for the dataset, source, and
runner; exact model names; retrieval scores; responses; and latency. These
details appear in [`run.json`](../results/evaluation/20260807T025557Z/run.json),
and the row-level observations appear in
[`results.csv`](../results/evaluation/20260807T025557Z/results.csv).

Automated scoring measured the fraction of expected concept groups present in
an answer. A response passed at 0.75 coverage or higher and failed if it added a
prohibited concept to an unanswerable response. This lexical score is a
repeatable screening measure, not a complete semantic judgment. Therefore, a
second review used the documented rubric: `2` for correct and complete, `1` for
partially correct or incomplete, and `0` for incorrect or missing. Groundedness
was evaluated for RAG answers by checking factual claims against the retrieved
context. This separation prevents lexical phrasing differences from being
mistaken for substantive errors.

## Results

| Metric | No RAG | RAG | Difference |
| --- | ---: | ---: | ---: |
| Automated pass rate | 6.7% | 80.0% | +73.3 points |
| Mean concept coverage | 0.35 | 0.88 | +0.53 |
| Rubric correctness | 56.7% | 90.0% | +33.3 points |
| Fully correct answers | 26.7% | 80.0% | +53.3 points |
| Median latency | 0.51 s | 0.57 s | +0.06 s |
| Mean latency | 0.51 s | 0.63 s | +0.12 s |

The RAG condition passed 12 of 15 automated checks; the no-RAG condition passed
only the temperature question. Retrieval found sufficient expected evidence for
14 of 15 questions, a 93.3% hit rate. The human rubric was intentionally less
binary. Without RAG, four answers were fully correct, nine were partially
correct, and two were incorrect, producing 17 of 30 possible points. With RAG,
12 answers were fully correct and three were partially correct, producing 27 of
30 points. All RAG answers remained factually grounded in their retrieved
context, including the incomplete responses.

For auditability, the no-RAG scores for questions 1–15 were
`0, 1, 2, 1, 0, 2, 2, 1, 1, 1, 2, 1, 1, 1, 1`; the corresponding RAG scores
were `2, 2, 2, 2, 2, 2, 2, 1, 2, 2, 2, 1, 1, 2, 2`.

The two unanswerable questions show the reliability benefit most clearly. RAG
declined both questions without adding outside explanations. The no-RAG answers
correctly began with “No,” but then asserted unsupported information about
cosine-similarity applications and transformer attention. Under a strict
course-grounding requirement, these are hallucination-handling failures even
though their opening conclusions were correct.

## Error analysis and debugging

Three recurring failure types emerged. First, direct generation often produced
plausible general knowledge instead of the assigned source's answer. For the
datum question, it interpreted _datum_ as a surveying reference and contrasted
it with measurements; this was coherent but wrong for the course definition.
For the industry-importance question, it listed common benefits of data while
omitting the section's causal explanation: growth in digital data, computing
capacity, and large datasets.

Second, successful retrieval did not guarantee a complete response. On question
12, the RAG answer stated that transformation makes systematic analysis possible
but omitted the conversion to consistent fields or records. On question 13, it
identified growth in digital data and computing capacity but omitted the
resulting large datasets, insights, decisions, and innovation. These answers
were grounded but incomplete, so prompt quality and answer completeness must be
tested separately from retrieval.

Third, question 8 was a retrieval miss. The top-three context supported the
finite `Yes` and `No` labels, but it did not provide enough evidence for the
second required concept: nominal labels have no inherent order. The generated
answer repeated the available evidence and therefore received partial credit.
This case should remain a regression test for experiments with chunk boundaries,
overlap, `top_k`, or reranking.

An earlier run at `20260807T025235Z` revealed an operational failure: all 30
generation requests returned HTTP 404 while embedding and retrieval continued.
The resulting artifacts showed 0% pass rates and near-zero latency, values that
could be misread as model results. The successful run used the same dataset,
source, runner hash, and configuration, which isolates the earlier outcome to
the external model-service state. The evaluator now exits unsuccessfully when
any model request returns an error and explicitly warns that such scores must
not be interpreted as model quality. Offline regression tests continue to check
dataset validation, deterministic chunking, scoring, abstention logic, retrieval
failure classification, and artifact generation.

## Reliability practices and limitations

The experiment suggests several practices for reliable local RAG evaluation:

- Version the dataset and hash the source, runner, and configuration.
- Hold model settings constant when comparing retrieval conditions.
- Separate retrieval hit rate, generation correctness, groundedness, latency,
  and operational availability rather than collapsing them into one score.
- Preserve failed-run artifacts for debugging, but fail the command and exclude
  them from quality comparisons.
- Convert each observed failure into a stable regression case.
- Manually review automated lexical scores, especially for incomplete but valid
  paraphrases and answers that begin correctly before adding unsupported facts.

The findings are limited to one short, instructor-prepared source, 15 questions,
one local chat model, one embedding model, and one hardware environment. The
questions were not independently authored or held out from all development, and
the rubric review is subjective. A single deterministic run does not estimate
variation across model seeds. Future work should use a larger held-out question
set, multiple reviewers, repeated runs, and targeted experiments on `top_k`,
chunk boundaries, and reranking. Within these limits, the evidence supports the
conclusion that retrieval substantially improves source-specific correctness
and abstention behavior at a small latency cost, while operational checks and
manual error analysis remain necessary for trustworthy deployment.

## Reproduction

With Ollama running and both configured models installed, reproduce the
evaluation from the repository root:

```sh
make evaluate-test
make evaluate
```

The successful run's automated summary is available in
[`summary.md`](../results/evaluation/20260807T025557Z/summary.md). A new live run
creates a separate timestamped directory so that prior evidence is not
overwritten.
