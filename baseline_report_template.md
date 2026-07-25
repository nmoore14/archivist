# Archivist Baseline Model Report

## Architecture and justification

This experiment compares `qwen3:0.6b` under two conditions: direct local question answering and retrieval-augmented question answering. The RAG condition cleans and divides a local study text into overlapping character chunks, embeds each chunk with `nomic-embed-text`, embeds each question with the same model, and selects the top three chunks by cosine similarity. The retrieved chunks are placed in a constrained prompt that requires chunk citations and permits an explicit “not enough information” response. Both conditions use the same chat model and temperature, so retrieval context is the main experimental difference.

The models are intentionally small and local because Archivist targets schools and small organizations with privacy and infrastructure constraints. The design uses Ollama for straightforward local inference, NumPy/scikit-learn for transparent retrieval, pandas for auditable result records, and manual scoring to avoid treating lexical overlap as human-level correctness.

## Initial results

Run the notebook, complete manual scoring, and paste the generated contents of `results/report_summary.txt` here. Report the number of scored questions, correctness percentage, fully correct percentage, Retrieval Hit Rate@3, average groundedness, hallucination handling, latency, and approximate Python RSS change. Do not report placeholder or example numbers.

> Results summary: _Not yet generated. Complete the experiment and manual review first._

Interpret the measurements conservatively. Note whether errors followed retrieval misses, whether the model used the supplied evidence, and whether the two out-of-scope questions were handled without unsupported claims.

## Limitations and potential improvements

This is a small initial baseline based on one section and 15 researcher-written questions. Manual scores can vary by reviewer, character chunks can split concepts, and process RSS excludes much of Ollama’s separate runtime memory. The source text should be checked against an instructor-approved export.

Potential follow-up work includes improved PDF/HTML preprocessing, token-aware chunking, chunk-size and overlap sweeps, larger top-k values, metadata filters, reranking, stronger local chat models, alternative embeddings, a larger independently authored question set, multiple human evaluators, RAGAS-assisted evaluation, Docker-based memory monitoring, and integration with the broader Archivist application.
