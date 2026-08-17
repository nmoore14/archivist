import importlib.util
import argparse
import json
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[2] / "scripts" / "evaluate.py"
SPEC = importlib.util.spec_from_file_location("archivist_evaluate", SCRIPT)
evaluate = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
sys.modules[SPEC.name] = evaluate
SPEC.loader.exec_module(evaluate)


class EvaluationTests(unittest.TestCase):
    def test_versioned_dataset_is_valid(self):
        dataset = evaluate.load_dataset(evaluate.DEFAULT_DATASET)
        self.assertEqual(dataset["dataset_version"], "openstax-1.3-v1")
        self.assertEqual(len(dataset["cases"]), 15)
        self.assertEqual(len({case["id"] for case in dataset["cases"]}), 15)

    def test_chunking_is_deterministic_and_overlaps(self):
        text = "A" * 80 + ". " + "B" * 80 + ". " + "C" * 80
        first = evaluate.chunk_text(text, 100, 20)
        second = evaluate.chunk_text(text, 100, 20)
        self.assertEqual(first, second)
        self.assertGreater(len(first), 1)
        self.assertTrue(set(first[0][-20:]) & set(first[1][:20]))

    def test_chunking_rejects_invalid_configuration(self):
        for size, overlap in ((0, 0), (100, -1), (100, 100)):
            with self.subTest(size=size, overlap=overlap):
                with self.assertRaises(ValueError):
                    evaluate.chunk_text("content", size, overlap)

    def test_concept_coverage_accepts_alternatives(self):
        concepts = [["schema", "consistent fields"], ["rows and columns", "tabular"]]
        self.assertEqual(evaluate.concept_coverage("Consistent fields in a tabular layout.", concepts), 1.0)
        self.assertEqual(evaluate.concept_coverage("The schema is fixed.", concepts), 0.5)

    def test_unanswerable_case_rejects_hallucinated_details(self):
        case = {
            "answerable": False,
            "required_concepts": [["does not", "not provide"]],
            "forbidden_concepts": ["self-attention"],
        }
        coverage, passed, category = evaluate.classify(
            case,
            "The section does not provide this, but self-attention creates the vectors.",
            True,
            "",
        )
        self.assertEqual(coverage, 1.0)
        self.assertFalse(passed)
        self.assertEqual(category, "failed_abstention")

    def test_retrieval_miss_takes_precedence(self):
        case = {"answerable": True, "required_concepts": [["expected"]]}
        _, passed, category = evaluate.classify(case, "unrelated", False, "")
        self.assertFalse(passed)
        self.assertEqual(category, "retrieval_miss")

    def test_duplicate_case_ids_are_rejected(self):
        data = {
            "dataset_version": "test-v1",
            "source": "data/openstax_section_1_3.txt",
            "cases": [
                {"id": "same", "category": "test", "question": "Q", "reference_answer": "A", "answerable": True, "required_concepts": [["a"]]},
                {"id": "same", "category": "test", "question": "Q2", "reference_answer": "A2", "answerable": True, "required_concepts": [["a2"]]},
            ],
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "cases.json"
            path.write_text(json.dumps(data))
            with self.assertRaisesRegex(ValueError, "duplicate case ID"):
                evaluate.load_dataset(path)

    def test_output_bundle_contains_reproducibility_and_review_files(self):
        dataset = evaluate.load_dataset(evaluate.DEFAULT_DATASET)
        args = argparse.Namespace(
            dataset=evaluate.DEFAULT_DATASET,
            chat_model="test-chat",
            embed_model="test-embed",
            ollama_url="http://localhost:11434",
            seed=42,
            chunk_size=1000,
            chunk_overlap=200,
            top_k=3,
        )
        result = evaluate.Result(
            case_id="q01", category="definition", condition="rag", question="Q",
            reference_answer="A", answerable=True, answer="A", latency_seconds=0.1,
            lexical_coverage=1.0, automated_pass=True, retrieval_hit=True,
            retrieved_chunk_ids=[0], retrieved_scores=[0.9], retrieved_context="context",
            error_category="pass",
        )
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "run"
            evaluate.write_outputs(output, dataset, args, [result])
            self.assertEqual(
                {path.name for path in output.iterdir()},
                {"run.json", "results.csv", "manual-review.csv", "summary.md"},
            )
            metadata = json.loads((output / "run.json").read_text())["metadata"]
            self.assertEqual(metadata["seed"], 42)
            self.assertIn("runner_sha256", metadata)


if __name__ == "__main__":
    unittest.main()
