"""Core extraction engine — manages paper analysis sessions."""

from __future__ import annotations

import json
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

from . import prompts


@dataclass
class PaperRecord:
    """Stores the structured extraction for a single paper."""

    filename: str
    raw_extraction: str = ""
    table_row: str = ""
    metadata: dict = field(default_factory=dict)


class ExtractionSession:
    """Manages an extraction session across multiple papers.

    Accumulates PaperRecords and can produce consolidated outputs
    (merged table, synthesis prompt, etc.).
    """

    def __init__(self) -> None:
        self.papers: list[PaperRecord] = []

    # ------------------------------------------------------------------
    # Building messages for the LLM
    # ------------------------------------------------------------------

    @staticmethod
    def system_message() -> dict:
        """Return the system message for the Claude API."""
        return {"role": "system", "content": prompts.SYSTEM_PROMPT}

    @staticmethod
    def analysis_messages(paper_text: str) -> list[dict]:
        """Return user messages to trigger analysis of *paper_text*."""
        return [
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": paper_text},
                    {"type": "text", "text": prompts.ANALYSIS_USER_PROMPT},
                ],
            }
        ]

    def synthesis_messages(self) -> list[dict]:
        """Return messages to request a cross-paper synthesis."""
        context = self._all_extractions_context()
        return [
            {
                "role": "user",
                "content": (
                    f"Here are the extractions so far:\n\n{context}\n\n"
                    f"{prompts.SYNTHESIS_PROMPT}"
                ),
            }
        ]

    def expansion_messages(self) -> list[dict]:
        """Return messages to request related-paper expansion."""
        context = self._all_extractions_context()
        return [
            {
                "role": "user",
                "content": (
                    f"Here are the extractions so far:\n\n{context}\n\n"
                    f"{prompts.RELATED_EXPANSION_PROMPT}"
                ),
            }
        ]

    # ------------------------------------------------------------------
    # Recording results
    # ------------------------------------------------------------------

    def add_paper(self, filename: str, raw_extraction: str) -> PaperRecord:
        """Parse an LLM extraction response and store it."""
        record = PaperRecord(
            filename=filename,
            raw_extraction=raw_extraction,
            table_row=self._extract_table_row(raw_extraction),
        )
        self.papers.append(record)
        return record

    # ------------------------------------------------------------------
    # Consolidated outputs
    # ------------------------------------------------------------------

    def consolidated_table(self) -> str:
        """Return a Markdown table merging all STEP-8 rows."""
        rows = [prompts.TABLE_HEADER_ROW, prompts.TABLE_SEPARATOR]
        for paper in self.papers:
            if paper.table_row:
                rows.append(paper.table_row)
        return "\n".join(rows)

    def export_table_csv(self, path: Path) -> None:
        """Write the consolidated table as CSV."""
        import csv

        with open(path, "w", newline="", encoding="utf-8") as fh:
            writer = csv.writer(fh)
            writer.writerow(prompts.TABLE_HEADERS)
            for paper in self.papers:
                if paper.table_row:
                    cells = [
                        c.strip()
                        for c in paper.table_row.strip("|").split("|")
                    ]
                    writer.writerow(cells)

    def save_session(self, path: Path) -> None:
        """Persist the session to a JSON file."""
        data = {
            "papers": [
                {
                    "filename": p.filename,
                    "raw_extraction": p.raw_extraction,
                    "table_row": p.table_row,
                }
                for p in self.papers
            ]
        }
        path.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")

    @classmethod
    def load_session(cls, path: Path) -> "ExtractionSession":
        """Restore a session from a JSON file."""
        data = json.loads(path.read_text(encoding="utf-8"))
        session = cls()
        for entry in data["papers"]:
            record = PaperRecord(
                filename=entry["filename"],
                raw_extraction=entry["raw_extraction"],
                table_row=entry.get("table_row", ""),
            )
            session.papers.append(record)
        return session

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    @staticmethod
    def _extract_table_row(text: str) -> str:
        """Pull the STEP 8 table row from the raw extraction."""
        for line in text.splitlines():
            stripped = line.strip()
            if stripped.startswith("|") and stripped.endswith("|"):
                cells = [c.strip() for c in stripped.strip("|").split("|")]
                # Skip header / separator rows
                if cells and not all(c in ("---", "") for c in cells):
                    if cells[0] not in ("Ref",):
                        return stripped
        return ""

    def _all_extractions_context(self) -> str:
        parts: list[str] = []
        for i, paper in enumerate(self.papers, 1):
            parts.append(f"=== Paper {i}: {paper.filename} ===")
            parts.append(paper.raw_extraction)
            parts.append("")
        return "\n".join(parts)
