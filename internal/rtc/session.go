package rtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	rtctokenbuilder "github.com/AgoraIO/Tools/DynamicKey/AgoraDynamicKey/go/src/rtctokenbuilder2"
	agorartc "github.com/zyy17/agora-server-sdk/agora/rtc"

	"go-trans/internal/agentx"
	"go-trans/internal/asr"
	"go-trans/internal/audio"
	"go-trans/internal/deepseek"
)

const (
	roleSender   = "sender"
	roleReceiver = "receiver"
	roleDuplex   = "duplex"
)

type Config struct {
	Role       string
	SourceLang string
	TargetLang string

	AppID   string
	AppCert string
	Token   string
	Channel string
	UID     string

	ASRBaseURL   string
	TTSCommand   string
	AutoStartASR bool
	ASRStartCmd  string

	EnableAgent       bool
	AgentKnowledgeDir string
	AgentReportDir    string
	MCPContextURL     string
}

type Message struct {
	Type       string `json:"type"`
	ReqID      string `json:"req_id"`
	FromUID    string `json:"from_uid"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
	ASRText    string `json:"asr_text"`
	TransText  string `json:"trans_text"`
	TS         int64  `json:"ts"`
}

type session struct {
	cfg          Config
	asrCli       *asr.Client
	trans        *deepseek.Client
	ctxHist      []string
	sessionAgent *agentx.SessionAgent

	connected chan struct{}
	onceConn  sync.Once
}

func Run(cfg Config) error {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return err
	}

	if normalized.Role == roleSender || normalized.Role == roleDuplex {
		if err := ensureASRReady(normalized); err != nil {
			return err
		}
	}

	rand.Seed(time.Now().UnixNano())

	s := &session{
		cfg:          normalized,
		asrCli:       asr.NewClient(),
		trans:        deepseek.NewClient(),
		ctxHist:      make([]string, 0, 5),
		sessionAgent: nil,
		connected:    make(chan struct{}),
	}
	s.asrCli.BaseURL = s.cfg.ASRBaseURL
	if s.cfg.EnableAgent {
		s.sessionAgent = agentx.NewSessionAgent(agentx.Options{
			SessionID:    fmt.Sprintf("%s_%s", s.cfg.Channel, s.cfg.UID),
			KnowledgeDir: s.cfg.AgentKnowledgeDir,
			ReportDir:    s.cfg.AgentReportDir,
			MaxRagDocs:   3,
			MCPEndpoint:  s.cfg.MCPContextURL,
		}, s.trans)
		defer func() {
			if err := s.sessionAgent.Close(); err != nil {
				log.Printf("close session agent failed: %v", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Keep runtime console focused on remote translation messages.

	return s.run(ctx)
}

func normalizeConfig(cfg Config) (Config, error) {
	cfg.Role = strings.ToLower(strings.TrimSpace(cfg.Role))
	if cfg.Role == "" {
		cfg.Role = roleSender
	}
	if cfg.Role != roleSender && cfg.Role != roleReceiver && cfg.Role != roleDuplex {
		return cfg, fmt.Errorf("invalid role: %s", cfg.Role)
	}

	if strings.TrimSpace(cfg.Channel) == "" {
		cfg.Channel = os.Getenv("AGORA_CHANNEL")
	}
	if strings.TrimSpace(cfg.UID) == "" {
		cfg.UID = os.Getenv("AGORA_UID")
	}
	if strings.TrimSpace(cfg.AppID) == "" {
		cfg.AppID = os.Getenv("AGORA_APP_ID")
	}
	if strings.TrimSpace(cfg.AppCert) == "" {
		cfg.AppCert = os.Getenv("AGORA_APP_CERT")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		cfg.Token = os.Getenv("AGORA_TOKEN")
	}

	if strings.TrimSpace(cfg.Channel) == "" {
		return cfg, errors.New("empty channel, use --channel or AGORA_CHANNEL")
	}
	if strings.TrimSpace(cfg.UID) == "" {
		return cfg, errors.New("empty uid, use --uid or AGORA_UID")
	}
	if strings.TrimSpace(cfg.AppID) == "" {
		return cfg, errors.New("empty app id, use --app-id or AGORA_APP_ID")
	}
	if strings.TrimSpace(cfg.Token) == "" && strings.TrimSpace(cfg.AppCert) == "" {
		return cfg, errors.New("token and app cert are both empty; provide --token/AGORA_TOKEN or --app-cert/AGORA_APP_CERT")
	}

	if strings.TrimSpace(cfg.SourceLang) == "" {
		cfg.SourceLang = "auto"
	}
	if strings.TrimSpace(cfg.TargetLang) == "" {
		cfg.TargetLang = "en"
	}
	if strings.TrimSpace(cfg.ASRBaseURL) == "" {
		cfg.ASRBaseURL = "http://localhost:8000"
	}

	if strings.TrimSpace(cfg.ASRStartCmd) == "" {
		cfg.ASRStartCmd = strings.TrimSpace(os.Getenv("ASR_START_CMD"))
	}
	if strings.TrimSpace(cfg.ASRStartCmd) == "" {
		cfg.ASRStartCmd = "./scripts/start_asr.sh"
	}

	if strings.TrimSpace(cfg.AgentKnowledgeDir) == "" {
		cfg.AgentKnowledgeDir = strings.TrimSpace(os.Getenv("AGENT_KNOWLEDGE_DIR"))
	}
	if strings.TrimSpace(cfg.AgentKnowledgeDir) == "" {
		cfg.AgentKnowledgeDir = "knowledge"
	}

	if strings.TrimSpace(cfg.AgentReportDir) == "" {
		cfg.AgentReportDir = strings.TrimSpace(os.Getenv("AGENT_REPORT_DIR"))
	}
	if strings.TrimSpace(cfg.AgentReportDir) == "" {
		cfg.AgentReportDir = "reports"
	}

	if strings.TrimSpace(cfg.MCPContextURL) == "" {
		cfg.MCPContextURL = strings.TrimSpace(os.Getenv("AGENT_MCP_CONTEXT_URL"))
	}

	return cfg, nil
}

func (s *session) run(ctx context.Context) error {
	token, err := s.resolveToken()
	if err != nil {
		return err
	}

	connCfg := &agorartc.RtcConnectionConfig{
		AutoSubscribeAudio:            false,
		AutoSubscribeVideo:            false,
		EnableAudioRecordingOrPlayout: false,
		ClientRole:                    agorartc.ClientRoleBroadcaster,
		ChannelProfile:                agorartc.ChannelProfileLiveBroadcasting,
	}

	publishCfg := agorartc.NewRtcConPublishConfig()
	publishCfg.IsPublishAudio = false
	publishCfg.IsPublishVideo = false
	publishCfg.AudioPublishType = agorartc.AudioPublishTypeNoPublish
	publishCfg.VideoPublishType = agorartc.VideoPublishTypeNoPublish

	svcCfg := agorartc.NewAgoraServiceConfig()
	svcCfg.AppId = s.cfg.AppID
	if ret := agorartc.Initialize(svcCfg); ret != 0 {
		return fmt.Errorf("initialize agora service failed: %d", ret)
	}
	defer agorartc.Release()

	conn := agorartc.NewRtcConnection(connCfg, publishCfg)
	defer conn.Release()

	conn.RegisterObserver(&agorartc.RtcConnectionObserver{
		OnConnected: func(rtcConn *agorartc.RtcConnection, info *agorartc.RtcConnectionInfo, reason int) {
			s.onceConn.Do(func() { close(s.connected) })
		},
		OnDisconnected: func(rtcConn *agorartc.RtcConnection, info *agorartc.RtcConnectionInfo, reason int) {
		},
		OnUserJoined: func(rtcConn *agorartc.RtcConnection, uid string) {
		},
		OnUserLeft: func(rtcConn *agorartc.RtcConnection, uid string, reason int) {
		},
	})

	conn.RegisterLocalUserObserver(&agorartc.LocalUserObserver{
		OnStreamMessage: func(localUser *agorartc.LocalUser, uid string, streamID int, data []byte) {
			s.handleMessage(data, uid)
		},
	})

	if ret := conn.Connect(token, s.cfg.Channel, s.cfg.UID); ret != 0 {
		return fmt.Errorf("connect rtc failed: %d", ret)
	}
	defer conn.Disconnect()

	select {
	case <-s.connected:
	case <-time.After(15 * time.Second):
		return errors.New("rtc connection timeout")
	}

	if s.cfg.Role == roleSender || s.cfg.Role == roleDuplex {
		go s.senderLoop(ctx, conn)
	}

	<-ctx.Done()
	s.finalizeSessionReport()
	return nil
}

func (s *session) resolveToken() (string, error) {
	if strings.TrimSpace(s.cfg.Token) != "" {
		return s.cfg.Token, nil
	}

	token, err := rtctokenbuilder.BuildTokenWithUserAccount(
		s.cfg.AppID,
		s.cfg.AppCert,
		s.cfg.Channel,
		s.cfg.UID,
		rtctokenbuilder.RolePublisher,
		3600,
		3600,
	)
	if err != nil {
		return "", fmt.Errorf("build token failed: %w", err)
	}
	return token, nil
}

func (s *session) senderLoop(ctx context.Context, conn *agorartc.RtcConnection) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wavPath, err := makeTempWavPath()
		if err != nil {
			log.Printf("create temp wav failed: %v", err)
			continue
		}

		err = recordWav(wavPath)
		if err != nil {
			if strings.Contains(err.Error(), "no speech detected") {
				_ = os.Remove(wavPath)
				continue
			}
			log.Printf("record wav failed: %v", err)
			_ = os.Remove(wavPath)
			continue
		}

		asrText, err := s.asrCli.TranscribeStream(wavPath, s.cfg.SourceLang, func(ev asr.Event) {
			if strings.EqualFold(ev.Type, "partial") {
				partialText := strings.TrimSpace(ev.Text)
				if partialText != "" {
					// fmt.Printf("[LOCAL-PARTIAL] %s\n", partialText)
				}
			}
		})
		_ = os.Remove(wavPath)
		if err != nil {
			log.Printf("asr failed: %v", err)
			continue
		}
		asrText = strings.TrimSpace(asrText)
		if asrText == "" {
			continue
		}

		translated, err := s.trans.Translate(s.contextString(asrText), asrText, s.cfg.TargetLang)
		if err != nil {
			log.Printf("translate failed: %v", err)
			continue
		}
		translated = strings.TrimSpace(translated)
		s.pushContext(asrText)

		msg := Message{
			Type:       "translation",
			ReqID:      newReqID(),
			FromUID:    s.cfg.UID,
			SourceLang: s.cfg.SourceLang,
			TargetLang: s.cfg.TargetLang,
			ASRText:    asrText,
			TransText:  translated,
			TS:         time.Now().UnixMilli(),
		}

		if s.sessionAgent != nil {
			s.sessionAgent.AddTurn(agentx.Turn{
				SpeakerID:      s.cfg.UID,
				SourceLang:     s.cfg.SourceLang,
				TargetLang:     s.cfg.TargetLang,
				OriginalText:   msg.ASRText,
				TranslatedText: msg.TransText,
				TimestampMs:    msg.TS,
			})
		}

		bs, _ := json.Marshal(msg)
		if ret := conn.SendStreamMessage(bs); ret != 0 {
			log.Printf("send stream message failed: %d", ret)
			continue
		}

		// fmt.Printf("[LOCAL] %s -> %s\n", msg.ASRText, msg.TransText)
	}
}

func (s *session) handleMessage(data []byte, fromUID string) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("invalid rtc message from=%s: %v", fromUID, err)
		return
	}

	if msg.Type == "" {
		msg.Type = "translation"
	}
	if msg.FromUID == "" {
		msg.FromUID = fromUID
	}

	fmt.Printf("[REMOTE][%s] %s -> %s\n", msg.FromUID, msg.ASRText, msg.TransText)

	if s.sessionAgent != nil {
		s.sessionAgent.AddTurn(agentx.Turn{
			SpeakerID:      msg.FromUID,
			SourceLang:     msg.SourceLang,
			TargetLang:     msg.TargetLang,
			OriginalText:   msg.ASRText,
			TranslatedText: msg.TransText,
			TimestampMs:    msg.TS,
		})
	}

	if s.cfg.Role == roleReceiver || s.cfg.Role == roleDuplex {
		s.runTTS(msg)
	}
}

func (s *session) runTTS(msg Message) {
	if s.cfg.TTSCommand == "" {
		return
	}

	cmd := exec.Command("sh", "-c", s.cfg.TTSCommand)
	cmd.Env = append(os.Environ(),
		"TTS_TEXT="+msg.TransText,
		"TTS_LANG="+msg.TargetLang,
		"TTS_FROM_UID="+msg.FromUID,
	)
	out, err := cmd.CombinedOutput()
	// Keep RTC console clean by default; show TTS command output only when explicitly enabled.
	if len(out) > 0 && strings.EqualFold(strings.TrimSpace(os.Getenv("RTC_TTS_VERBOSE")), "1") {
		fmt.Print(string(out))
	}
	if err != nil {
		log.Printf("tts command failed: %v", err)
	}
}

func (s *session) pushContext(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	s.ctxHist = append(s.ctxHist, text)
	if len(s.ctxHist) > 5 {
		s.ctxHist = s.ctxHist[len(s.ctxHist)-5:]
	}
}

func (s *session) contextString(currentText string) string {
	hist := strings.Join(s.ctxHist, "\n")

	ragInfo := ""
	if s.cfg.EnableAgent && s.sessionAgent != nil {
		ragInfo = strings.TrimSpace(s.sessionAgent.RetrieveRAG(currentText))
	}

	if ragInfo == "" || ragInfo == "(none)" {
		return hist
	}

	return fmt.Sprintf("会话近况:\n%s\n\n相关专业术语或背景(供参考):\n%s", hist, ragInfo)
}

func newReqID() string {
	return fmt.Sprintf("%d-%06d", time.Now().UnixMilli(), rand.Intn(1000000))
}

func makeTempWavPath() (string, error) {
	file, err := os.CreateTemp("", "rtc_chunk_*.wav")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	return filepath.Clean(name), nil
}

func recordWav(filePath string) error {
	return audio.RecordWav(filePath)
}

func (s *session) finalizeSessionReport() {
	if !s.cfg.EnableAgent || s.sessionAgent == nil {
		return
	}

	path, err := s.sessionAgent.SaveTurns()
	if err != nil {
		log.Printf("save turns failed: %v", err)
		return
	}
	if path == "" {
		return
	}

	fmt.Printf("\n[AGENT] Session ended. Generating report...\n")

	reportDir := strings.TrimSpace(s.cfg.AgentReportDir)
	if reportDir == "" {
		reportDir = "reports"
	}

	// Create a background process to generate the report offline,
	// preventing it from being killed if the terminal/tmux window is closed.
	cmdArgs := []string{"report", "--input", path, "--report-dir", reportDir, "--cleanup-input"}
	if s.cfg.AgentKnowledgeDir != "" {
		cmdArgs = append(cmdArgs, "--knowledge", s.cfg.AgentKnowledgeDir)
	}
	if s.cfg.MCPContextURL != "" {
		cmdArgs = append(cmdArgs, "--mcp", s.cfg.MCPContextURL)
	}

	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		log.Printf("start background report failed: %v", err)
		return
	}

	fmt.Printf("[AGENT] Background report generation started. Output will be saved as session_n.json/md in: %s\n", reportDir)
}

func ensureASRReady(cfg Config) error {
	hostPort, err := asrHostPort(cfg.ASRBaseURL)
	if err != nil {
		return err
	}

	if isPortOpen(hostPort, 800*time.Millisecond) {
		return nil
	}

	if !cfg.AutoStartASR {
		return fmt.Errorf("asr service is not reachable at %s", cfg.ASRBaseURL)
	}

	cmdText := strings.TrimSpace(cfg.ASRStartCmd)
	if cmdText == "" {
		return fmt.Errorf("asr service is not reachable and no start command configured")
	}

	fmt.Printf("[RTC] ASR not reachable at %s, starting with: %s\n", cfg.ASRBaseURL, cmdText)
	cmd := exec.Command("sh", "-c", cmdText)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start asr command failed: %w", err)
	}

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if isPortOpen(hostPort, 800*time.Millisecond) {
			fmt.Printf("[RTC] ASR is ready at %s\n", cfg.ASRBaseURL)
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}

	return fmt.Errorf("asr still unreachable at %s after auto-start", cfg.ASRBaseURL)
}

func asrHostPort(baseURL string) (string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = "http://localhost:8000"
	}
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}

	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid asr url: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := u.Port()
	if port == "" {
		if strings.EqualFold(u.Scheme, "https") || strings.EqualFold(u.Scheme, "wss") {
			port = "443"
		} else {
			port = "80"
		}
	}

	return net.JoinHostPort(host, port), nil
}

func isPortOpen(hostPort string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", hostPort, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
