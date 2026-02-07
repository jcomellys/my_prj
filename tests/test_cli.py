"""Tests for the CLI interface."""

import json
from pathlib import Path

import pytest

from literature_review_extractor.cli import main


class TestCLI:
    def test_no_command_shows_help(self, capsys):
        ret = main([])
        assert ret == 0
        captured = capsys.readouterr()
        assert "literature-review-extractor" in captured.out

    def test_version(self, capsys):
        with pytest.raises(SystemExit) as exc_info:
            main(["--version"])
        assert exc_info.value.code == 0

    def test_prompt_text(self, capsys):
        ret = main(["prompt"])
        assert ret == 0
        captured = capsys.readouterr()
        assert "extraction engine" in captured.out

    def test_prompt_json(self, capsys):
        ret = main(["prompt", "--format", "json"])
        assert ret == 0
        captured = capsys.readouterr()
        data = json.loads(captured.out)
        assert "system" in data

    def test_add_and_table(self, tmp_path: Path, capsys):
        session_file = tmp_path / "session.json"
        extraction_file = tmp_path / "extract.txt"
        extraction_file.write_text(
            "Some analysis\n"
            "| Auth2023 | 2023 | Sim | Sensor | Method | CV | Acc | 95% | None | None | kw |\n"
        )

        ret = main([
            "add", str(session_file),
            "--name", "Auth2023",
            "--file", str(extraction_file),
        ])
        assert ret == 0

        ret = main(["table", str(session_file)])
        assert ret == 0
        captured = capsys.readouterr()
        assert "Auth2023" in captured.out

    def test_add_csv_export(self, tmp_path: Path):
        session_file = tmp_path / "session.json"
        extraction_file = tmp_path / "extract.txt"
        extraction_file.write_text(
            "| Auth2023 | 2023 | Sim | Sensor | Method | CV | Acc | 95% | None | None | kw |\n"
        )
        main([
            "add", str(session_file),
            "--name", "Auth2023",
            "--file", str(extraction_file),
        ])

        csv_path = tmp_path / "out.csv"
        main(["table", str(session_file), "--csv", str(csv_path)])
        assert csv_path.exists()
        assert "Auth2023" in csv_path.read_text()

    def test_synthesize(self, tmp_path: Path, capsys):
        session_file = tmp_path / "session.json"
        extraction_file = tmp_path / "extract.txt"
        extraction_file.write_text("Some paper content\n")
        main([
            "add", str(session_file),
            "--name", "Test",
            "--file", str(extraction_file),
        ])

        ret = main(["synthesize", str(session_file)])
        assert ret == 0
        captured = capsys.readouterr()
        assert "dominant methods" in captured.out

    def test_expand(self, tmp_path: Path, capsys):
        session_file = tmp_path / "session.json"
        extraction_file = tmp_path / "extract.txt"
        extraction_file.write_text("Some paper content\n")
        main([
            "add", str(session_file),
            "--name", "Test",
            "--file", str(extraction_file),
        ])

        ret = main(["expand", str(session_file)])
        assert ret == 0
        captured = capsys.readouterr()
        assert "CONTRADICT" in captured.out
