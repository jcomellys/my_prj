"""
Panama Power Grid Dashboard - Main Application Entry Point.

Sistema de Análisis en Tiempo Real de Flujos de Potencia
del Sistema Interconectado Nacional (SIN) de Panamá a 230kV.

Run with: python main.py
Then open: http://localhost:8000
"""

import sys
import os

# Add project root to path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse
from fastapi.middleware.cors import CORSMiddleware

from backend.api.routes import router

app = FastAPI(
    title="Panama Power Grid Dashboard",
    description=(
        "Sistema de Análisis en Tiempo Real de Flujos de Potencia "
        "del Sistema Interconectado Nacional (SIN) de Panamá a 230kV"
    ),
    version="1.0.0",
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include API routes
app.include_router(router)

# Serve static files
app.mount("/css", StaticFiles(directory="frontend/css"), name="css")
app.mount("/js", StaticFiles(directory="frontend/js"), name="js")
app.mount("/assets", StaticFiles(directory="frontend/assets"), name="assets")


@app.get("/")
async def serve_dashboard():
    """Serve the main dashboard page."""
    return FileResponse("frontend/index.html")


if __name__ == "__main__":
    import uvicorn
    print("\n" + "=" * 60)
    print("  PANAMA POWER GRID DASHBOARD")
    print("  Sistema Interconectado Nacional - 230kV")
    print("=" * 60)
    print(f"  Dashboard: http://localhost:8000")
    print(f"  API Docs:  http://localhost:8000/docs")
    print("=" * 60 + "\n")

    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=8000,
        reload=True,
    )
