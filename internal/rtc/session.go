package rtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
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

	"mini-tmk-agent/internal/asr"
	"mini-tmk-agent/internal/audio"
	"mini-tmk-agent/internal/deepseek"
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

	ASRBaseURL string
	TTSCommand string
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
	cfg     Config
	asrCli  *asr.Client
	trans   *deepseek.Client
	ctxHist []string

	connected chan struct{}
	onceConn  sync.Once
}

func Run(cfg Config) error {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return err
	}

	rand.Seed(time.Now().UnixNano())

	s := &session{
		cfg:       normalized,
		asrCli:    asr.NewClient(),
		trans:     deepseek.NewClient(),
		ctxHist:   make([]string, 0, 5),
		connected: make(chan struct{}),
	}
	s.asrCli.BaseURL = s.cfg.ASRBaseURL

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if s.cfg.Role == roleSender || s.cfg.Role == roleDuplex {
		fmt.Printf("RTC sender started, channel=%s, uid=%s\n", s.cfg.Channel, s.cfg.UID)
	} else if s.cfg.Role == roleReceiver {
		fmt.Printf("RTC receiver started, channel=%s, uid=%s\n", s.cfg.Channel, s.cfg.UID)
	} else {
		fmt.Printf("RTC duplex started, channel=%s, uid=%s\n", s.cfg.Channel, s.cfg.UID)
	}

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
			fmt.Printf("RTC connected: channel=%s uid=%s reason=%d\n", info.ChannelId, info.LocalUserId, reason)
			s.onceConn.Do(func() { close(s.connected) })
		},
		OnDisconnected: func(rtcConn *agorartc.RtcConnection, info *agorartc.RtcConnectionInfo, reason int) {
			fmt.Printf("RTC disconnected: reason=%d\n", reason)
		},
		OnUserJoined: func(rtcConn *agorartc.RtcConnection, uid string) {
			fmt.Printf("Remote joined: uid=%s\n", uid)
		},
		OnUserLeft: func(rtcConn *agorartc.RtcConnection, uid string, reason int) {
			fmt.Printf("Remote left: uid=%s reason=%d\n", uid, reason)
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

		asrText, err := s.asrCli.Transcribe(wavPath, s.cfg.SourceLang)
		_ = os.Remove(wavPath)
		if err != nil {
			log.Printf("asr failed: %v", err)
			continue
		}
		asrText = strings.TrimSpace(asrText)
		if asrText == "" {
			continue
		}

		translated, err := s.trans.Translate(s.contextString(), asrText, s.cfg.TargetLang)
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

		bs, _ := json.Marshal(msg)
		if ret := conn.SendStreamMessage(bs); ret != 0 {
			log.Printf("send stream message failed: %d", ret)
			continue
		}

		fmt.Printf("[LOCAL] %s -> %s\n", msg.ASRText, msg.TransText)
	}
}

func (s *session) handleMessage(data []byte, fromUID string) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		fmt.Printf("[RTC RAW] from=%s data=%s\n", fromUID, string(data))
		return
	}

	if msg.Type == "" {
		msg.Type = "translation"
	}
	if msg.FromUID == "" {
		msg.FromUID = fromUID
	}

	fmt.Printf("[REMOTE][%s] %s -> %s\n", msg.FromUID, msg.ASRText, msg.TransText)

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
	if len(out) > 0 {
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

func (s *session) contextString() string {
	return strings.Join(s.ctxHist, "\n")
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
