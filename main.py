"""
Panama Power Grid Real-Time Dashboard
Main application entry point.

Usage:
    python main.py

Then open http://localhost:8000 in your browser.
"""

import os
import sys

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse
from fastapi.middleware.cors import CORSMiddleware

# Add project root to path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from backend.api.routes import router

app = FastAPI(
    title="Panama Power Grid Dashboard",
    description="Real-time power flow analysis and monitoring for Panama's 230kV Sistema Interconectado Nacional (SIN)",
    version="1.0.0",
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
    return {"status": "ok", "service": "Panama Power Grid Dashboard"}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=8000,
        reload=True,
    )
