package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/jing332/tts-server-go/service/azure"
	"github.com/jing332/tts-server-go/service/edge"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GracefulServer struct {
	Server       *http.Server
	serveMux     *http.ServeMux
	shutdownLoad chan struct{}

	edgeRwLock  sync.RWMutex
	azureRwLock sync.RWMutex
}

// HandleFunc 注册
func (s *GracefulServer) HandleFunc() {
	if s.serveMux == nil {
		s.serveMux = &http.ServeMux{}
	}
	s.serveMux.HandleFunc("/", s.webAPIHandler)
	s.serveMux.HandleFunc("/api/azure", s.azureAPIHandler)
	s.serveMux.HandleFunc("/api/ra", s.edgeAPIHandler)
	s.serveMux.HandleFunc("/api/legado", s.legadoAPIHandler)
}

// ListenAndServe 监听服务
func (s *GracefulServer) ListenAndServe(port int64) error {
	if s.shutdownLoad == nil {
		s.shutdownLoad = make(chan struct{})
	}

	s.Server = &http.Server{
		Addr:           ":" + strconv.FormatInt(port, 10),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        s.serveMux,
	}
	log.Infoln("服务已启动, 监听端口为", s.Server.Addr)
	err := s.Server.ListenAndServe()
	if err == http.ErrServerClosed { /*说明调用Shutdown关闭*/
		err = nil
	} else if err != nil {
		return err
	}

	<-s.shutdownLoad /*等待,直到服务关闭*/

	return nil
}

// Shutdown 关闭监听服务，需等待响应
func (s *GracefulServer) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := s.Server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("shutdown失败: %d", err)
	}

	close(s.shutdownLoad)
	s.shutdownLoad = nil

	return nil
}

//go:embed public/index.html
var indexHtml string

//go:embed public/azure.html
var azureHtml string

func (s *GracefulServer) webAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/azure.html" {
		w.Write([]byte(azureHtml))
	} else if r.URL.Path == "/index.html" || r.URL.Path == "/" {
		w.Write([]byte(indexHtml))
	} else {
		w.WriteHeader(http.StatusNotFound)
	}

}

func sendErrorMsg(w http.ResponseWriter, msg string) error {
	log.Warnln("获取音频失败:", msg)
	w.WriteHeader(http.StatusServiceUnavailable)

	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	return nil
}

// Microsoft Edge 大声朗读接口
func (s *GracefulServer) edgeAPIHandler(w http.ResponseWriter, r *http.Request) {
	s.edgeRwLock.Lock()
	defer s.edgeRwLock.Unlock()
	format := `webm-24khz-16bit-mono-opus`
	ctxType := `audio/webm; codec=opus`

	defer r.Body.Close()
	ssml, _ := io.ReadAll(r.Body)

	b, err := edge.GetAudioForRetry(string(ssml), format, 3)
	if err != nil {
		if e := sendErrorMsg(w, err.Error()); e != nil {
			log.Warnln("发送错误消息失败:", e)
		}
		return
	}
	w.Header().Set("Content-Type", ctxType)
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout=5")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))

	_, err = w.Write(b)
	if err != nil {
		log.Println("写入音频数据失败:", err)
		return
	}
}

// 微软Azure TTS接口
func (s *GracefulServer) azureAPIHandler(w http.ResponseWriter, r *http.Request) {
	s.azureRwLock.Lock()
	defer s.azureRwLock.Unlock()
	format := r.Header.Get("Format")

	defer r.Body.Close()
	ssml, _ := io.ReadAll(r.Body)

	b, err := azure.GetAudioForRetry(string(ssml), format, 3)
	if err != nil {
		if e := sendErrorMsg(w, err.Error()); e != nil {
			log.Warnln("发送错误消息失败:", e)
		}
		return
	}

	w.Header().Set("Content-Type", formatContentType(format))
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout=5")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))

	_, err = w.Write(b)
	if err != nil {
		log.Warnln("写入音频数据失败:", err)
		return
	}
}

func (s *GracefulServer) legadoAPIHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	apiUrl := params.Get("api")
	name := params.Get("name")
	voiceName := params.Get("voiceName")
	styleName := params.Get("styleName")
	styleDegree := params.Get("styleDegree")
	voiceFormat := params.Get("voiceFormat")

	jsonStr, err := genLegodoJson(apiUrl, name, voiceName, styleName, styleDegree, voiceFormat)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	}

	w.Write(jsonStr)
}

func genLegodoJson(api, name, voiceName, styleName, styleDegree, format string) ([]byte, error) {
	t := time.Now().UnixNano() / 1e6 //毫秒时间戳
	var url string
	if styleName == "" {
		url = api + ` ,{"method":"POST","body":"<speak xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" version=\"1.0\" xml:lang=\"en-US\"><voice name=\"` +
			voiceName + `\"><prosody rate=\"{{(speakSpeed -10) * 2}}%\" pitch=\"+0Hz\">{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;')}}</prosody></voice></speak>"}`
	} else {
		url = api + ` ,{"method":"POST","body":"<speak xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" version=\"1.0\" xml:lang=\"en-US\"><voice name=\"` +
			voiceName + `\"><mstts:express-as style=\"` + styleName + `\" styledegree=\" ` + styleDegree + `\"><prosody rate=\"{{(speakSpeed -10) * 2}}%\" pitch=\"+0Hz\">{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;')}}</prosody> </mstts:express-as></voice></speak>"}`
	}

	head := `{"Content-Type":"text/plain","Authorization":"Bearer ","Format":"` + format + `"}`
	legadoJson := &LegadoJson{Name: name, URL: url, ID: t, LastUpdateTime: t, ContentType: formatContentType(format), Header: head}

	body, err := json.Marshal(legadoJson)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func formatContentType(format string) string {
	t := strings.Split(format, "-")[0]
	switch t {
	case "audio":
		return "audio/mpeg"
	case "webm":
		return "audio/webm; codec=opus"
	case "ogg":
		return "audio/ogg; codecs=opus; rate=16000"
	case "riff":
		return "audio/x-wav"
	case "raw":
		if strings.HasSuffix(format, "truesilk") {
			return "audio/SILK"
		} else {
			return "audio/basic"
		}
	}
	return ""
}

type LegadoJson struct {
	ContentType    string `json:"contentType"`
	Header         string `json:"header"`
	ID             int64  `json:"id"`
	LastUpdateTime int64  `json:"lastUpdateTime"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	//ConcurrentRate   string `json:"concurrentRate"`
	//EnabledCookieJar bool   `json:"enabledCookieJar"`
	//LoginCheckJs   string `json:"loginCheckJs"`
	//LoginUI        string `json:"loginUi"`
	//LoginURL       string `json:"loginUrl"`

}