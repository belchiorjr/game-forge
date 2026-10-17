# Sprite service

Python service run natively on the Mac (GPU via PyTorch MPS). See references/sprites.md
of the game-forge-platform skill. Suggested start:

    python3 -m venv .venv && source .venv/bin/activate
    pip install torch diffusers transformers accelerate safetensors pillow fastapi uvicorn
