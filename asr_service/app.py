from fastapi import FastAPI, UploadFile, File, Form
from funasr import AutoModel
import tempfile
import re
import shutil
import uuid

app = FastAPI()

model = AutoModel(
    model="iic/SenseVoiceSmall",
    disable_update=True,
)

def clean_asr_text(text: str) -> str:
    text = re.sub(r"<\|.*?\|>", "", text)
    return text.strip()

@app.post("/transcribe")
async def transcribe(
    file: UploadFile = File(...),
    source_lang: str = Form("auto"),
    speaker_id: str = Form("unknown"),
    req_id: str = Form(None),
):
    req_id = req_id or str(uuid.uuid4())

    with tempfile.NamedTemporaryFile(delete=False, suffix=".wav") as temp_file:
        shutil.copyfileobj(file.file, temp_file)
        temp_path = temp_file.name

    result = model.generate(
        input=temp_path,
        language=source_lang,
    )

    raw_text = result[0]["text"]
    clean_text = clean_asr_text(raw_text)

    return {
        "req_id": req_id,
        "speaker_id": speaker_id,
        "text": clean_text
    }