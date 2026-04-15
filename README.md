# go-trans

一个轻量级的实时同声传译agent，支持多语言语音识别和翻译。

[![Go Version](https://img.shields.io/badge/Go-1.26.1-blue.svg)](https://golang.org/)
[![Python Version](https://img.shields.io/badge/Python-3.8+-green.svg)](https://www.python.org/)

## 简介

go-trans 是一个基于 Go 的实时同声传译工具，支持：
- **会话智能分析**：会话结束自动生成结构化复盘报告（摘要、观点、争议等）
- **实时流模式**：连续语音识别和翻译
- **转录模式**：音频文件转录和翻译
- **多语言支持**：中文、英文、日文、西班牙文等
- **Web UI**：现代化的浏览器界面
- **CLI 工具**：命令行操作
- **会话 Agent**：会话结束自动生成摘要与双方观点
- **ASR 服务**：基于 FunASR 的语音识别服务
- **翻译服务**：集成 DeepSeek API 的翻译功能
- **会话 Agent**：RAG + Skills + MCP 上下文增强

   

#### 克隆项目
```bash
git clone https://github.com/Fogivz/go-trans.git
cd go-trans
```

### 环境变量

- `DEEPSEEK_API_KEY`：DeepSeek API 密钥（必需）

```bash
echo 'export DEEPSEEK_API_KEY="your-api-key-here"' >> ~/.bashrc
source ~/.bashrc
```

## 部署方式
### 方式一：终端部署（CLI 模式）

适合命令行用户或集成到其他系统中。

#### 1. 构建项目
```bash
make install  # 安装到系统路径
```

#### 2. 启动 ASR 服务
```bash
./scripts/start_asr.sh
```

#### 3. 使用 CLI 调用命令

在新终端中：

```bash
# 实时流模式
go-trans stream --source-lang zh --target-lang en

# 支持的语言参数：
# --source-lang: zh(中文), en(英文), ja(日文), es(西班牙文)
# --target-lang: en(英文), zh(中文), ja(日文), es(西班牙文)

eg: go-trans transcript --file test.wav --output testout.txt

### 方式二：Web 部署（完整服务）
./start.sh
```

#### 2. 使用 Web 界面

1. 打开浏览器访问 http://localhost:8080
2. 选择工作模式：
3. 选择源语言和目标语言
4. 点击 "Start Translation" 开始
5. 说话或上传音频文件

### RTC 终端互通模式（Agora）

- 双向全双工（duplex）：每个终端都可同时说和收

#### RTC 环境变量
- `AGORA_TOKEN`：可选，若提供则优先使用
- `AGORA_CHANNEL`：频道名
- `AGORA_UID`：当前终端用户 ID（A/B 端必须不同）
- `DEEPSEEK_API_KEY`：翻译必需
- `AGENT_KNOWLEDGE_DIR`：本地知识库目录（默认 `knowledge`）
- `AGENT_REPORT_DIR`：会话报告输出目录（默认 `reports`）
- `AGENT_MCP_CONTEXT_URL`：可选 MCP 上下文 HTTP 地址（通过 `?query=` 取增强上下文）
1. 打开 Agora Console：https://console.agora.io/
2. 注册并登录账号。
3. 在控制台创建一个新项目（Project）。
4. 进入项目详情页后可查看 `App ID`。
5. 在项目配置中启用并查看 `App Certificate`（首次通常需要手动开启证书）。
6. 导出环境变量


#### 启用 Agora RTC

在项目中使用 agora-server-sdk 模块：

```bash
go get github.com/zyy17/agora-server-sdk
```

下载 Agora 库。将 Agora 库下载到当前目录的 agora_libs/ 文件夹：

```bash
curl -fsSL \
  https://raw.githubusercontent.com/zyy17/agora-server-sdk/refs/heads/main/scripts/download_agora_libs.sh | bash
```

使用 Agora 库构建：

```bash
CGO_LDFLAGS="-L$(pwd)/agora_libs -Wl,-rpath-link=$(pwd)/agora_libs" go build -tags rtc -o go-trans main.go
```

设置环境变量：

```bash
export AGORA_APP_ID="your_app_id"
export AGORA_APP_CERT="your_app_cert"
```

#### 一键启动 RTC 双端（tmux）

```bash
./scripts/start_rtc_pair.sh
```

如果提示 `tmux not found`，请先安装：

```bash
# Ubuntu/Debian
sudo apt update && sudo apt install -y tmux

# macOS
brew install tmux
```

该脚本会：

- 自动启动 ASR（8000）
- 自动创建 tmux 双窗格
- 在两个窗格分别启动 A/B 端 `rtc duplex`

可选环境变量：

- `AGORA_CHANNEL`（默认 `demo-room`）
- `UID_A`（默认 `1001`）
- `UID_B`（默认 `1002`）
- `LANG_A_SRC`/`LANG_A_TGT`（默认 `zh` -> `en`）
- `LANG_B_SRC`/`LANG_B_TGT`（默认 `en` -> `zh`）
- `ASR_URL`（默认 `http://localhost:8000`）


### 会话 Agent（RAG + MCP + Skills）

RTC 会话中，系统会持续记录双方 turn（原文和译文），并在会话结束时自动生成报告：

- `summary`：会话摘要
- `viewpoints`：双方观点、共识与争议
- `turns`：结构化会话记录

报告默认写入 `reports/` 目录。


RAG 工作方式：

- 从 `knowledge/`（或 `--agent-knowledge-dir`）递归读取 `.txt/.md/.json`
- 根据当前会话文本做关键词重叠检索，取 Top-K 片段作为增强上下文

MCP 工作方式：

- 若配置 `--agent-mcp-url` 或 `AGENT_MCP_CONTEXT_URL`，会在生成摘要/观点前获取外部上下文并注入 Prompt
- 支持两种 MCP 源：
   - HTTP 模式：例如 `http://localhost:9000/context`，请求时自动附带 `?query=...`
   - Stdio 模式：例如 `npx -y @modelcontextprotocol/server-memory`
- Stdio 模式会自动发现工具并优先尝试 `context/search/retrieve/query` 类工具
- MCP 获取失败时会自动降级为 `(none)`，不影响主流程

### 流模式

- 实时监听麦克风输入
- 自动分段识别和翻译
- 支持上下文记忆
- 可随时停止和重启

### 转录模式

- 上传 WAV 音频文件
- 一次性处理完整音频
- 生成带时间戳的转录结果


## 配置


### 音频设置

- 采样率：16kHz
- 格式：16-bit WAV
- 声道：单声道


### 添加新语言支持

1. 在 `asr_service/app.py` 的 `SUPPORTED_LANGUAGES` 中添加语言代码
2. 确保 FunASR 模型支持该语言
3. 更新翻译服务的目标语言列表


## 故障排除

### 常见问题

1. **ASR 服务启动失败**
   - 检查 Python 虚拟环境是否正确激活
   - 确认所有依赖已安装
   - 检查端口 8000 是否被占用

2. **音频录制失败**
   - 检查麦克风权限
   - 确认 PortAudio 已正确安装
   - 检查音频设备是否可用

3. **翻译服务错误**
   - 确认 DeepSeek API 密钥已设置
   - 检查网络连接
   - 验证 API 额度

4. **Web UI 无法访问**
   - 确认端口 8080 未被占用
   - 检查防火墙设置
   - 确认服务已启动



