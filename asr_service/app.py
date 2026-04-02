from fastapi import FastAPI, UploadFile, File, Form, HTTPException
from funasr import AutoModel
import tempfile
import re
import shutil

app = FastAPI()

# 支持的语言列表
SUPPORTED_LANGUAGES = {"zh", "en", "ja", "es", "auto"}

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
):
    if source_lang not in SUPPORTED_LANGUAGES:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported language: {source_lang}. Supported languages: zh, en, ja, es, auto",
        )

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
        "text": clean_text
    }