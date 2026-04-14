# go-trans

一个轻量级的实时同声传译agent，支持多语言语音识别和翻译。

[![Go Version](https://img.shields.io/badge/Go-1.26.1-blue.svg)](https://golang.org/)
[![Python Version](https://img.shields.io/badge/Python-3.8+-green.svg)](https://www.python.org/)

## 简介

go-trans 是一个基于 Go 的实时同声传译工具，支持：

- **实时流模式**：连续语音识别和翻译
- **转录模式**：音频文件转录和翻译
- **多语言支持**：中文、英文、日文、西班牙文等
- **Web UI**：现代化的浏览器界面
- **CLI 工具**：命令行操作

## 架构

```
mini-tmk-agent/
├── main.go                # 主入口
├── cmd/                   # CLI 命令
│   ├── root.go            # 根命令
│   └── stream.go          # 流模式命令
    └── transcript.go      # 文件转录模式命令
├── internal/              # 内部模块
│   ├── agent/             # 代理核心逻辑
│   │   ├── interpreter_agent.go  # 代理实现
│   │   ├── pipeline.go           # 处理管道
│   │   ├── steps.go              # 管道步骤
│   │   └── memory.go             # 上下文记忆
│   ├── asr/               # 语音识别客户端
│   │   └── client.go      # ASR 服务客户端
│   ├── audio/             # 音频处理
│   │   └── recorder.go    # WAV 录音器
│   └── deepseek/          # 翻译服务
│       └── client.go      # DeepSeek API 客户端
├── web/                   # Web 界面
│   ├── main.go           # Web 服务器
│   └── index.html        # 前端界面
├── asr_service/          # Python ASR 服务
│   ├── app.py            # FastAPI 应用
│   ├── requirements.txt  # Python 依赖
│   └── venv/             # 虚拟环境
├── Makefile              # 构建脚本
├── start.sh              # 启动脚本
└── go.mod                # Go 模块
```

### 核心组件

- **代理引擎 (Go)**：基于管道模式的处理引擎
- **ASR 服务 (Python)**：基于 FunASR 的语音识别服务
- **翻译服务**：集成 DeepSeek API 的翻译功能
- **Web 界面**：Gin + WebSocket 的实时界面
- **音频处理**：PortAudio 驱动的实时录音


### 依赖安装

1. **安装 Go**：
   ```bash
   # Ubuntu/Debian
   sudo apt update
   sudo apt install golang-go

   # macOS
   brew install go

   # 或从官网下载：https://golang.org/dl/
   ```

2. **安装 Python 和依赖**：
   ```bash
   # Ubuntu/Debian
   sudo apt install python3 python3-pip python3-venv

   # macOS
   brew install python3
   ```

3. **音频库**：
   ```bash
   # Ubuntu/Debian
   sudo apt install portaudio19-dev

   # macOS
   brew install portaudio
   ```

#### 克隆项目
```bash
git clone https://github.com/Fogivz/mini-tmk-agent.git
cd mini-tmk-agent
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
cd asr_service

# 创建虚拟环境
python3 -m venv venv
source venv/bin/activate

# 安装依赖
pip install -r requirements.txt

# 启动服务
uvicorn app:app --port 8000
```

#### 3. 使用 CLI 调用命令

在新终端中：

```bash
# 实时流模式
mini-tmk-agent stream --source-lang zh --target-lang en

# 支持的语言参数：
# --source-lang: zh(中文), en(英文), ja(日文), es(西班牙文)
# --target-lang: en(英文), zh(中文), ja(日文), es(西班牙文)

#文件转录模式
mini-tmk-agent transcript --file <your-audio-file> --output <destination-file-path> 
```
eg: mini-tmk-agent transcript --file test.wav --output testout.txt

### 方式二：Web 部署（完整服务）

提供完整的 Web 界面和 API 服务。


#### 1. 一键启动
```bash
./start.sh
```

脚本将自动：
- 创建 Python 虚拟环境
- 安装依赖
- 启动 ASR 服务（端口 8000）
- 启动 Web UI（端口 8080）

#### 2. 使用 Web 界面

1. 打开浏览器访问 http://localhost:8080
2. 选择工作模式：
   - **Stream Mode**：实时翻译
   - **Transcript Mode**：文件转录
3. 选择源语言和目标语言
4. 点击 "Start Translation" 开始
5. 说话或上传音频文件
6. 点击 "Stop Translation" 停止

## 使用说明

### RTC 终端互通模式（Agora）

新增 `rtc` 命令后，可在两个终端通过同一个 RTC channel 进行实时翻译文本互传：

- A 端（sender）：麦克风录音 -> ASR -> DeepSeek 翻译 -> RTC DataStream 发送
- B 端（receiver）：接收翻译文本并在终端输出
- 双向全双工（duplex）：每个终端都可同时说和收

#### RTC 环境变量

- `AGORA_APP_ID`：Agora App ID
- `AGORA_APP_CERT`：Agora App Certificate（当不传 `AGORA_TOKEN` 时用于自动签发 token）
- `AGORA_TOKEN`：可选，若提供则优先使用
- `AGORA_CHANNEL`：频道名
- `AGORA_UID`：当前终端用户 ID（A/B 端必须不同）
- `DEEPSEEK_API_KEY`：翻译必需

#### 如何获取 Agora App ID 和 App Certificate

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

# Linux
```bash
CGO_LDFLAGS="-L$(pwd)/agora_libs -Wl,-rpath-link=$(pwd)/agora_libs" go build -tags rtc -o mini-tmk-agent main.go
```


#### 双终端运行示例

终端 A（发送端）：

```bash
# 使用动态库路径运行：
export LD_LIBRARY_PATH=$(pwd)/agora_libs

export AGORA_APP_ID="your_app_id"
export AGORA_APP_CERT="your_app_cert"
export AGORA_CHANNEL="demo-room"
export AGORA_UID="1001"
export DEEPSEEK_API_KEY="your_deepseek_key"

./mini-tmk-agent rtc --role sender --source-lang zh --target-lang en --asr-url http://localhost:8000
```

终端 B（接收端，仅显示文本）：

```bash
# 使用动态库路径运行：
export LD_LIBRARY_PATH=$(pwd)/agora_libs

export AGORA_APP_ID="your_app_id"
export AGORA_APP_CERT="your_app_cert"
export AGORA_CHANNEL="demo-room"
export AGORA_UID="1002"

./mini-tmk-agent rtc --role receiver --source-lang zh --target-lang en 
```


#### 双向全双工示例（A/B 同时说和收）

终端 A：

```bash
export AGORA_APP_ID="your_app_id"
export AGORA_APP_CERT="your_app_cert"
export AGORA_CHANNEL="demo-room"
export AGORA_UID="1001"
export DEEPSEEK_API_KEY="your_deepseek_key"

./mini-tmk-agent rtc --role duplex --source-lang zh --target-lang en --asr-url http://localhost:8000
```

终端 B：

```bash
export AGORA_APP_ID="your_app_id"
export AGORA_APP_CERT="your_app_cert"
export AGORA_CHANNEL="demo-room"
export AGORA_UID="terminal-b"
export DEEPSEEK_API_KEY="1002"

./mini-tmk-agent rtc --role duplex --source-lang en --target-lang zh --asr-url http://localhost:8000
```

说明：A/B 使用同一个 channel，但 UID 必须不同；两端都使用 `duplex` 即可实现全双工实时翻译。

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

### 日志查看

```bash
# 查看 Go 服务日志
go run web/main.go

# 查看 ASR 服务日志
cd asr_service && python -m uvicorn app:app --log-level info
```


## 致谢

- [FunASR](https://github.com/alibaba-damo-academy/FunASR) - 语音识别
- [DeepSeek](https://platform.deepseek.com/) - 翻译服务
- [Gin](https://gin-gonic.com/) - Go Web 框架
- [FastAPI](https://fastapi.tiangolo.com/) - Python Web 框架
