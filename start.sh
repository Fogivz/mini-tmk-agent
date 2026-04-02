#!/bin/bash

echo "Starting Mini TMK Agent services..."

# Start ASR service in background
echo "Starting ASR service on port 8000..."
cd asr_service

# Create venv if not exists
if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv venv
fi

source venv/bin/activate

# Install dependencies if requirements.txt exists
if [ -f "requirements.txt" ]; then
    echo "Installing Python dependencies..."
    pip install -r requirements.txt
fi

python -m uvicorn app:app --reload --port 8000 &
ASR_PID=$!

# Wait a bit for ASR to start
sleep 2

# Start Web UI
echo "Starting Web UI on port 8080..."
cd ../web
go run main.go 2>/dev/null &
WEB_PID=$!

echo "Services started!"
echo "ASR service PID: $ASR_PID"
echo "Web UI PID: $WEB_PID"
echo ""
echo "Access Web UI at: http://localhost:8080"
echo "ASR API at: http://localhost:8000"
echo ""
echo "Press Ctrl+C to stop all services"

# Wait for interrupt
trap "echo 'Stopping services...'; kill $ASR_PID $WEB_PID; exit" INT
wait