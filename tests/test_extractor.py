"""Tests for the extraction session logic."""

import json
import textwrap
from pathlib import Path

import pytest

from literature_review_extractor.extractor import ExtractionSession, PaperRecord
from literature_review_extractor.prompts import TABLE_HEADERS


SAMPLE_EXTRACTION = textwrap.dedent("""\
    STEP 1 — METADATA
    Title: Fault Detection in Rotating Machinery Using Vibration Analysis
    Authors: Smith, J.; Lee, K.
    Year: 2023
    Venue: IEEE Transactions on Industrial Electronics
    DOI: 10.1109/TIE.2023.000001
    Application domain: Predictive maintenance
    Type of study: experimental

    STEP 2 — PROBLEM AND CONTRIBUTION
    Problem addressed: Early fault detection in bearings is unreliable with traditional methods.
    Main contribution:
    - Novel wavelet-CNN hybrid for bearing fault classification
    - Real-time inference on edge hardware
    Novelty vs prior work: First to combine CWT with lightweight CNN on embedded device.

    STEP 3 — DATA CHARACTERISTICS
    Data source: Real plant
    Physical variables measured: vibration acceleration
    Sensors: accelerometer (PCB 352C33)
    Sampling frequency: 12 kHz
    Signal duration: 10 s per sample
    Dataset size: 4800 samples (4 classes)
    Pre-processing: CWT, normalization
    Feature extraction: scalogram images

    STEP 4 — METHOD
    Signal processing methods: Continuous wavelet transform
    Model / algorithm: CNN (MobileNetV2)
    Training strategy: transfer learning, 80/20 split
    Validation method: 5-fold cross-validation
    Baseline comparison: SVM, Random Forest, standard CNN

    STEP 5 — RESULTS
    Performance metrics: accuracy, F1-score
    Numerical values: accuracy 98.7%, F1 0.986
    Improvement over baseline: +3.2% accuracy vs SVM
    Operational feasibility: real-time (15 ms inference)

    STEP 6 — LIMITATIONS
    A) Declared by authors:
    - Single machine type tested
    - Controlled lab-like conditions

    B) Critical weaknesses (your analysis):
    - No cross-domain validation
    - Small dataset may overfit

    STEP 7 — RELATION TO OTHER WORK
    (omitted for brevity)

    STEP 8 — TABLE ROW OUTPUT
    | Smith2023 | 2023 | Vibration | Accelerometer | CWT+CNN | 5-fold CV | Accuracy | 98.7% | Single machine | No cross-domain test | fault detection, CNN, wavelet |
""")


class TestExtractionSession:
    def test_add_paper_extracts_table_row(self):
        session = ExtractionSession()
        record = session.add_paper("Smith2023.pdf", SAMPLE_EXTRACTION)

        assert record.filename == "Smith2023.pdf"
        assert record.table_row.startswith("| Smith2023")
        assert "98.7%" in record.table_row

    def test_consolidated_table_includes_header(self):
        session = ExtractionSession()
        session.add_paper("Smith2023.pdf", SAMPLE_EXTRACTION)
        table = session.consolidated_table()

        lines = table.strip().splitlines()
        assert lines[0].startswith("| Ref")
        assert lines[1].startswith("| ---")
        assert lines[2].startswith("| Smith2023")

    def test_save_and_load_roundtrip(self, tmp_path: Path):
        session = ExtractionSession()
        session.add_paper("Smith2023.pdf", SAMPLE_EXTRACTION)

        path = tmp_path / "session.json"
        session.save_session(path)

        loaded = ExtractionSession.load_session(path)
        assert len(loaded.papers) == 1
        assert loaded.papers[0].filename == "Smith2023.pdf"
        assert loaded.papers[0].table_row == session.papers[0].table_row

    def test_export_csv(self, tmp_path: Path):
        session = ExtractionSession()
        session.add_paper("Smith2023.pdf", SAMPLE_EXTRACTION)

        csv_path = tmp_path / "output.csv"
        session.export_table_csv(csv_path)

        content = csv_path.read_text()
        lines = content.strip().splitlines()
        assert lines[0] == ",".join(TABLE_HEADERS)
        assert "Smith2023" in lines[1]

    def test_system_message_format(self):
        msg = ExtractionSession.system_message()
        assert msg["role"] == "system"
        assert "extraction engine" in msg["content"]

    def test_analysis_messages(self):
        msgs = ExtractionSession.analysis_messages("Some paper text here.")
        assert len(msgs) == 1
        assert msgs[0]["role"] == "user"

    def test_synthesis_messages_include_papers(self):
        session = ExtractionSession()
        session.add_paper("A.pdf", SAMPLE_EXTRACTION)
        msgs = session.synthesis_messages()
        assert "Paper 1: A.pdf" in msgs[0]["content"]
        assert "dominant methods" in msgs[0]["content"]

    def test_expansion_messages_include_papers(self):
        session = ExtractionSession()
        session.add_paper("A.pdf", SAMPLE_EXTRACTION)
        msgs = session.expansion_messages()
        assert "Paper 1: A.pdf" in msgs[0]["content"]
        assert "CONTRADICT" in msgs[0]["content"]

    def test_empty_extraction_no_table_row(self):
        session = ExtractionSession()
        record = session.add_paper("empty.pdf", "No structured content here.")
        assert record.table_row == ""

    def test_multiple_papers(self):
        session = ExtractionSession()
        session.add_paper("A.pdf", SAMPLE_EXTRACTION)

        second = SAMPLE_EXTRACTION.replace("Smith2023", "Lee2024").replace("2023", "2024")
        session.add_paper("B.pdf", second)

        table = session.consolidated_table()
        assert table.count("\n") >= 3  # header + sep + 2 data rows
