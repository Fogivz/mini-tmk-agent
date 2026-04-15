from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from funasr import AutoModel
import tempfile
import re
import os
import uuid
import base64

app = FastAPI()


@app.post("/transcribe")
async def transcribe_http(payload: dict):
    source_lang = payload.get("source_lang", "auto")
    speaker_id = payload.get("speaker_id", "unknown")
    audio_base64 = payload.get("audio_base64")
    if not audio_base64:
        return {
            "type": "error",
            "speaker_id": speaker_id,
            "text": "",
            "error": "audio_base64 is required",
        }

    try:
        audio_bytes = base64.b64decode(audio_base64)
        clean_text = transcribe_bytes(audio_bytes, source_lang)
        # 保持向后兼容：旧客户端通常只读取 text 字段。
        return {
            "type": "final",
            "speaker_id": speaker_id,
            "text": clean_text,
            "is_final": True,
        }
    except Exception as exc:
        return {
            "type": "error",
            "speaker_id": speaker_id,
            "text": "",
            "error": str(exc),
        }

model = AutoModel(
    model="iic/SenseVoiceSmall",
    disable_update=True,
)

def clean_asr_text(text: str) -> str:
    text = re.sub(r"<\|.*?\|>", "", text)
    return text.strip()


def transcribe_bytes(audio_bytes: bytes, source_lang: str) -> str:
    temp_path = ""
    try:
        with tempfile.NamedTemporaryFile(delete=False, suffix=".wav") as temp_file:
            temp_file.write(audio_bytes)
            temp_path = temp_file.name

        result = model.generate(
            input=temp_path,
            language=source_lang,
        )

        raw_text = result[0]["text"]
        return clean_asr_text(raw_text)
    finally:
        if temp_path and os.path.exists(temp_path):
            os.remove(temp_path)

@app.websocket("/ws/transcribe")
async def transcribe(websocket: WebSocket):
    await websocket.accept()
    sessions = {}

    try:
        while True:
            payload = await websocket.receive_json()

            msg_type = payload.get("type", "oneshot")
            source_lang = payload.get("source_lang", "auto")
            speaker_id = payload.get("speaker_id", "unknown")
            req_id = payload.get("req_id") or str(uuid.uuid4())
            audio_base64 = payload.get("audio_base64")

            try:
                if msg_type == "start":
                    sessions[req_id] = {
                        "source_lang": source_lang,
                        "speaker_id": speaker_id,
                        "audio": bytearray(),
                        "chunks": 0,
                        "last_partial": "",
                    }
                    continue

                if msg_type == "chunk":
                    state = sessions.get(req_id)
                    if not state:
                        await websocket.send_json({
                            "type": "error",
                            "req_id": req_id,
                            "speaker_id": speaker_id,
                            "text": "",
                            "error": "missing session, send start first"
                        })
                        continue

                    if not audio_base64:
                        await websocket.send_json({
                            "type": "error",
                            "req_id": req_id,
                            "speaker_id": state["speaker_id"],
                            "text": "",
                            "error": "audio_base64 is required for chunk"
                        })
                        continue

                    chunk_bytes = base64.b64decode(audio_base64)
                    state["audio"].extend(chunk_bytes)
                    state["chunks"] += 1

                    # Emit lightweight partial updates while receiving stream.
                    if state["chunks"] % 3 == 0 and len(state["audio"]) > 4096:
                        partial_text = transcribe_bytes(bytes(state["audio"]), state["source_lang"])
                        if partial_text and partial_text != state["last_partial"]:
                            state["last_partial"] = partial_text
                            await websocket.send_json({
                                "type": "partial",
                                "req_id": req_id,
                                "speaker_id": state["speaker_id"],
                                "text": partial_text,
                                "is_final": False,
                            })
                    continue

                if msg_type == "end":
                    state = sessions.pop(req_id, None)
                    if not state:
                        await websocket.send_json({
                            "type": "error",
                            "req_id": req_id,
                            "speaker_id": speaker_id,
                            "text": "",
                            "error": "missing session, send start first"
                        })
                        continue

                    final_text = transcribe_bytes(bytes(state["audio"]), state["source_lang"])
                    await websocket.send_json({
                        "type": "final",
                        "req_id": req_id,
                        "speaker_id": state["speaker_id"],
                        "text": final_text,
                        "is_final": True,
                    })
                    continue

                # Backward-compatible one-shot mode.
                if not audio_base64:
                    await websocket.send_json({
                        "type": "error",
                        "req_id": req_id,
                        "speaker_id": speaker_id,
                        "text": "",
                        "error": "audio_base64 is required"
                    })
                    continue

                audio_bytes = base64.b64decode(audio_base64)
                clean_text = transcribe_bytes(audio_bytes, source_lang)
                await websocket.send_json({
                    "type": "final",
                    "req_id": req_id,
                    "speaker_id": speaker_id,
                    "text": clean_text,
                    "is_final": True,
                })
            except Exception as exc:
                await websocket.send_json({
                    "type": "error",
                    "req_id": req_id,
                    "speaker_id": speaker_id,
                    "text": "",
                    "error": str(exc)
                })
    except WebSocketDisconnect:
        return