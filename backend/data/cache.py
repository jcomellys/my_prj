"""
SQLite cache layer for CND generation data.

Stores historical snapshots so the app can:
- Serve data even when CND is unreachable
- Build historical trend charts
- Reduce load on the external source

Database: data/generation_cache.db (auto-created)
"""

import json
import sqlite3
import logging
import os
from datetime import datetime, timedelta
from typing import Optional

logger = logging.getLogger(__name__)

# Default DB path next to this file
DB_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "db")
DB_PATH = os.path.join(DB_DIR, "generation_cache.db")


class GenerationCache:
    """SQLite-backed cache for generation snapshots."""

    def __init__(self, db_path: str = DB_PATH):
        self.db_path = db_path
        os.makedirs(os.path.dirname(db_path), exist_ok=True)
        self._init_db()

    def _init_db(self):
        """Create tables if they don't exist."""
        with sqlite3.connect(self.db_path) as conn:
            conn.execute("""
                CREATE TABLE IF NOT EXISTS generation_snapshots (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    cnd_timestamp TEXT,
                    fetched_at TEXT NOT NULL,
                    total_mw REAL,
                    hidro_mw REAL,
                    termica_mw REAL,
                    solar_mw REAL,
                    eolica_mw REAL,
                    raw_json TEXT NOT NULL,
                    summary_json TEXT NOT NULL
                )
            """)
            conn.execute("""
                CREATE INDEX IF NOT EXISTS idx_fetched_at
                ON generation_snapshots(fetched_at)
            """)
            conn.commit()
        logger.info("Generation cache initialized at %s", self.db_path)

    def store(self, raw_data: dict, summary_data: dict):
        """Store a generation snapshot."""
        totals = raw_data.get("totals", {})
        with sqlite3.connect(self.db_path) as conn:
            conn.execute(
                """
                INSERT INTO generation_snapshots
                    (cnd_timestamp, fetched_at, total_mw, hidro_mw,
                     termica_mw, solar_mw, eolica_mw, raw_json, summary_json)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    raw_data.get("timestamp"),
                    datetime.now().isoformat(),
                    raw_data.get("total_mw", 0),
                    totals.get("hidro", 0),
                    totals.get("termica", 0),
                    totals.get("solar", 0),
                    totals.get("eolica", 0),
                    json.dumps(raw_data, ensure_ascii=False),
                    json.dumps(summary_data, ensure_ascii=False),
                ),
            )
            conn.commit()

    def get_latest(self) -> Optional[dict]:
        """Get the most recent cached snapshot summary."""
        with sqlite3.connect(self.db_path) as conn:
            row = conn.execute(
                "SELECT summary_json FROM generation_snapshots "
                "ORDER BY id DESC LIMIT 1"
            ).fetchone()
        if row:
            return json.loads(row[0])
        return None

    def get_latest_raw(self) -> Optional[dict]:
        """Get the most recent cached raw data."""
        with sqlite3.connect(self.db_path) as conn:
            row = conn.execute(
                "SELECT raw_json FROM generation_snapshots "
                "ORDER BY id DESC LIMIT 1"
            ).fetchone()
        if row:
            return json.loads(row[0])
        return None

    def get_history(self, hours: int = 24) -> list[dict]:
        """Get historical total generation for the last N hours."""
        cutoff = (datetime.now() - timedelta(hours=hours)).isoformat()
        with sqlite3.connect(self.db_path) as conn:
            rows = conn.execute(
                """
                SELECT cnd_timestamp, fetched_at, total_mw,
                       hidro_mw, termica_mw, solar_mw, eolica_mw
                FROM generation_snapshots
                WHERE fetched_at > ?
                ORDER BY fetched_at ASC
                """,
                (cutoff,),
            ).fetchall()

        return [
            {
                "cnd_timestamp": r[0],
                "fetched_at": r[1],
                "total_mw": r[2],
                "hidro_mw": r[3],
                "termica_mw": r[4],
                "solar_mw": r[5],
                "eolica_mw": r[6],
            }
            for r in rows
        ]

    def get_snapshot_count(self) -> int:
        """Get total number of stored snapshots."""
        with sqlite3.connect(self.db_path) as conn:
            row = conn.execute(
                "SELECT COUNT(*) FROM generation_snapshots"
            ).fetchone()
        return row[0] if row else 0

    def cleanup_old(self, keep_days: int = 7):
        """Delete snapshots older than keep_days."""
        cutoff = (datetime.now() - timedelta(days=keep_days)).isoformat()
        with sqlite3.connect(self.db_path) as conn:
            result = conn.execute(
                "DELETE FROM generation_snapshots WHERE fetched_at < ?",
                (cutoff,),
            )
            conn.commit()
            if result.rowcount > 0:
                logger.info("Cleaned up %d old snapshots", result.rowcount)


# Module-level singleton
_cache: Optional[GenerationCache] = None


def get_cache() -> GenerationCache:
    """Get or create the singleton cache instance."""
    global _cache
    if _cache is None:
        _cache = GenerationCache()
    return _cache
