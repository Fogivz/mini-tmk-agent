from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from funasr import AutoModel
import tempfile
import re
import os
import uuid
import base64

app = FastAPI()

model = AutoModel(
    model="iic/SenseVoiceSmall",
    disable_update=True,
)

def clean_asr_text(text: str) -> str:
    text = re.sub(r"<\|.*?\|>", "", text)
    return text.strip()

@app.websocket("/ws/transcribe")
async def transcribe(websocket: WebSocket):
    await websocket.accept()

    try:
        while True:
            payload = await websocket.receive_json()

            source_lang = payload.get("source_lang", "auto")
            speaker_id = payload.get("speaker_id", "unknown")
            req_id = payload.get("req_id") or str(uuid.uuid4())
            audio_base64 = payload.get("audio_base64")

            if not audio_base64:
                await websocket.send_json({
                    "req_id": req_id,
                    "speaker_id": speaker_id,
                    "text": "",
                    "error": "audio_base64 is required"
                })
                continue

            temp_path = ""
            try:
                audio_bytes = base64.b64decode(audio_base64)

                with tempfile.NamedTemporaryFile(delete=False, suffix=".wav") as temp_file:
                    temp_file.write(audio_bytes)
                    temp_path = temp_file.name

                result = model.generate(
                    input=temp_path,
                    language=source_lang,
                )

                raw_text = result[0]["text"]
                clean_text = clean_asr_text(raw_text)

                await websocket.send_json({
                    "req_id": req_id,
                    "speaker_id": speaker_id,
                    "text": clean_text
                })
            except Exception as exc:
                await websocket.send_json({
                    "req_id": req_id,
                    "speaker_id": speaker_id,
                    "text": "",
                    "error": str(exc)
                })
            finally:
                if temp_path and os.path.exists(temp_path):
                    os.remove(temp_path)
    except WebSocketDisconnect:
        return