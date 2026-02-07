"""Command-line interface for the literature review extractor."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import __version__
from .extractor import ExtractionSession
from .prompts import SYSTEM_PROMPT, SYNTHESIS_PROMPT, RELATED_EXPANSION_PROMPT


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="literature-review-extractor",
        description="Structured extraction of academic papers for systematic literature reviews.",
    )
    parser.add_argument(
        "--version", action="version", version=f"%(prog)s {__version__}"
    )

    sub = parser.add_subparsers(dest="command")

    # --- prompt ---------------------------------------------------------
    prompt_p = sub.add_parser(
        "prompt",
        help="Print the system prompt to stdout (for manual use with Claude).",
    )
    prompt_p.add_argument(
        "--format",
        choices=["text", "json"],
        default="text",
        help="Output format (default: text).",
    )

    # --- table ----------------------------------------------------------
    table_p = sub.add_parser(
        "table",
        help="Show consolidated comparison table from a session file.",
    )
    table_p.add_argument(
        "session_file",
        type=Path,
        help="Path to a session JSON file.",
    )
    table_p.add_argument(
        "--csv",
        type=Path,
        default=None,
        help="Export table as CSV to this path.",
    )

    # --- add ------------------------------------------------------------
    add_p = sub.add_parser(
        "add",
        help="Add a raw extraction (paste from Claude) to the session.",
    )
    add_p.add_argument(
        "session_file",
        type=Path,
        help="Path to the session JSON file (created if missing).",
    )
    add_p.add_argument(
        "--name",
        required=True,
        help="Short identifier for the paper.",
    )
    add_p.add_argument(
        "--file",
        type=Path,
        default=None,
        help="Read extraction from a text file instead of stdin.",
    )

    # --- synthesize -----------------------------------------------------
    synth_p = sub.add_parser(
        "synthesize",
        help="Print the synthesis prompt pre-filled with session data.",
    )
    synth_p.add_argument(
        "session_file",
        type=Path,
        help="Path to a session JSON file.",
    )

    # --- expand ---------------------------------------------------------
    expand_p = sub.add_parser(
        "expand",
        help="Print the related-paper expansion prompt pre-filled with session data.",
    )
    expand_p.add_argument(
        "session_file",
        type=Path,
        help="Path to a session JSON file.",
    )

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv)

    if args.command is None:
        parser.print_help()
        return 0

    if args.command == "prompt":
        if args.format == "json":
            import json

            print(json.dumps({"system": SYSTEM_PROMPT}, ensure_ascii=False, indent=2))
        else:
            print(SYSTEM_PROMPT)
        return 0

    if args.command == "table":
        session = ExtractionSession.load_session(args.session_file)
        table = session.consolidated_table()
        print(table)
        if args.csv:
            session.export_table_csv(args.csv)
            print(f"\nCSV exported to {args.csv}", file=sys.stderr)
        return 0

    if args.command == "add":
        if args.session_file.exists():
            session = ExtractionSession.load_session(args.session_file)
        else:
            session = ExtractionSession()

        if args.file:
            text = args.file.read_text(encoding="utf-8")
        else:
            print("Paste the extraction text (Ctrl-D to finish):", file=sys.stderr)
            text = sys.stdin.read()

        record = session.add_paper(args.name, text)
        session.save_session(args.session_file)
        print(f"Added '{args.name}' — total papers: {len(session.papers)}", file=sys.stderr)
        if record.table_row:
            print(record.table_row)
        else:
            print("Warning: no table row (STEP 8) detected in extraction.", file=sys.stderr)
        return 0

    if args.command == "synthesize":
        session = ExtractionSession.load_session(args.session_file)
        msgs = session.synthesis_messages()
        print(msgs[0]["content"])
        return 0

    if args.command == "expand":
        session = ExtractionSession.load_session(args.session_file)
        msgs = session.expansion_messages()
        print(msgs[0]["content"])
        return 0

    parser.print_help()
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
