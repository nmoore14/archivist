# Automated evaluation summary

Dataset: `openstax-1.3-v1` (15 questions)

| Condition | Automated pass rate | Mean lexical coverage | Median latency |
| --- | ---: | ---: | ---: |
| `no_rag` | 6.7% | 0.35 | 0.51 s |
| `rag` | 80.0% | 0.88 | 0.57 s |

Retrieval hit rate: 93.3%

The automated pass score is a reproducible screening metric based on expected concepts and abstention behavior. It is not a substitute for the human correctness and groundedness review in `manual-review.csv`.
