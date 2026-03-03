"""
Panama Power Grid Real-Time Dashboard
Main application entry point.

Usage:
    python main.py

Then open http://localhost:8000 in your browser.

Architecture:
    - Background CND collector fetches real data every 60s from sitr.cnd.com.pa
    - Data is cached in SQLite for history and resilience
    - API serves real data when available, falls back to simulator
    - WebSocket pushes updates every 5s to connected clients
"""

import os
import sys
import asyncio
import logging

from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse
from fastapi.middleware.cors import CORSMiddleware

# Add project root to path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from backend.api.routes import router
from backend.data.cnd_collector import get_collector
from backend.data.cache import get_cache
from config.settings import CND_COLLECTOR_ENABLED, CND_COLLECTOR_INTERVAL_SECONDS

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger(__name__)


async def _collector_with_cache():
    """
    Background task: run the CND collector and store each fetch in SQLite.
    This runs as long as the app is alive.
    """
    collector = get_collector()
    collector.interval = CND_COLLECTOR_INTERVAL_SECONDS
    cache = get_cache()

    logger.info(
        "Starting CND collector (interval=%ds, cache_db=%s)",
        collector.interval, cache.db_path,
    )

    collector._running = True
    while collector._running:
        await collector._fetch_once()

        # If we got new data, store it in the cache
        raw = collector.get_latest_raw()
        summary = collector.get_latest()
        if raw and summary:
            try:
                cache.store(raw, summary)
            except Exception:
                logger.exception("Failed to store snapshot in cache")

        # Periodic cleanup (every ~100 fetches)
        if collector._fetch_count % 100 == 0 and collector._fetch_count > 0:
            cache.cleanup_old(keep_days=7)

        await asyncio.sleep(collector.interval)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage background tasks on startup/shutdown."""
    collector_task = None

    if CND_COLLECTOR_ENABLED:
        collector_task = asyncio.create_task(_collector_with_cache())
        logger.info("CND background collector started")
    else:
        logger.info("CND collector disabled, using simulator only")

    yield

    # Shutdown
    if collector_task:
        collector = get_collector()
        collector.stop()
        collector_task.cancel()
        logger.info("CND collector stopped")


app = FastAPI(
    title="Panama Power Grid Dashboard",
    description="Real-time power flow analysis and monitoring for Panama's 230kV Sistema Interconectado Nacional (SIN)",
    version="1.1.0",
    lifespan=lifespan,
)

# CORS middleware (required for WebSocket connections)
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include API routes
app.include_router(router)

# Serve static files - mount each subdirectory to match HTML paths
frontend_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "frontend")
app.mount("/css", StaticFiles(directory=os.path.join(frontend_dir, "css")), name="css")
app.mount("/js", StaticFiles(directory=os.path.join(frontend_dir, "js")), name="js")
app.mount("/assets", StaticFiles(directory=os.path.join(frontend_dir, "assets")), name="assets")


@app.get("/")
async def serve_dashboard():
    """Serve the main dashboard HTML page."""
    return FileResponse(os.path.join(frontend_dir, "index.html"))


@app.get("/health")
async def health_check():
    """Health check with collector status."""
    collector = get_collector()
    return {
        "status": "ok",
        "service": "Panama Power Grid Dashboard",
        "collector": collector.get_status(),
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=8000,
        reload=True,
    )
