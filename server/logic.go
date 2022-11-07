package server

import (
	"bytes"
	"encoding/json"
	"github.com/jing332/tts-server-go/service"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

type LegadoJson struct {
	ContentType    string `json:"contentType"`
	Header         string `json:"header"`
	ID             int64  `json:"id"`
	LastUpdateTime int64  `json:"lastUpdateTime"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	ConcurrentRate string `json:"concurrentRate"`
	//EnabledCookieJar bool   `json:"enabledCookieJar"`
	//LoginCheckJs   string `json:"loginCheckJs"`
	//LoginUI        string `json:"loginUi"`
	//LoginURL       string `json:"loginUrl"`
}

type CreationJson struct {
	Text        string `json:"text"`
	VoiceName   string `json:"voiceName"`
	VoiceId     string `json:"voiceId"`
	Rate        string `json:"rate"`
	Volume      string `json:"volume"`
	Style       string `json:"style"`
	StyleDegree string `json:"styleDegree"`
	Role        string `json:"role"`
	Format      string `json:"format"`
}

func (c *CreationJson) VoiceProperty() *service.VoiceProperty {
	rate, err := strconv.ParseInt(removePcmChar(c.Rate), 10, 8)
	if err != nil {
		log.Errorf("转换语速失败：%s", c.Rate)
		rate = 0
		err = nil
	}

	volume, err := strconv.ParseInt(removePcmChar(c.Volume), 10, 8)
	if err != nil {
		log.Errorf("转换音量失败：%s", c.Volume)
		volume = 0
		err = nil
	}
	styleDegree, err := strconv.ParseFloat(c.StyleDegree, 10)
	if err != nil {
		log.Errorf("转换风格强度失败：%s", c.StyleDegree)
		volume = 0
		err = nil
	}

	prosody := service.Prosody{Rate: int8(rate), Volume: int8(volume)}
	expressAs := service.ExpressAs{Style: c.Style, StyleDegree: float32(styleDegree), Role: c.Role}
	return &service.VoiceProperty{VoiceName: c.VoiceName, VoiceId: c.VoiceId, Prosody: prosody, ExpressAs: expressAs}
}

// 移除字符串中 % 符号
func removePcmChar(s string) string {
	return strings.ReplaceAll(s, "%", "")
}

//func FromVoiceProperty(pro *service.VoiceProperty) *CreationJson {
//	return &CreationJson{VoiceName: pro.VoiceName, VoiceId: pro.VoiceId, Rate: strconv.Itoa(pro.Rate), Volume: pro.Volume,
//		Style: pro.Style, StyleDegree: pro.StyleDegree, Role: pro.Role}
//}

/* 生成阅读APP朗读朗读引擎Json (Edge, Azure) */
func genLegodoJson(api, name, voiceName, styleName, styleDegree, roleName, voiceFormat, token, concurrentRate string) ([]byte, error) {
	t := time.Now().UnixNano() / 1e6 //毫秒时间戳
	var url string
	if styleName == "" { /* Edge大声朗读 */
		url = api + ` ,{"method":"POST","body":"<speak xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" version=\"1.0\" xml:lang=\"en-US\"><voice name=\"` +
			voiceName + `\"><prosody rate=\"{{(speakSpeed -10) * 2}}%\" pitch=\"+0Hz\">{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/\\/g, '')}}</prosody></voice></speak>"}`
	} else { /* Azure TTS */
		url = api + ` ,{"method":"POST","body":"<speak xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" version=\"1.0\" xml:lang=\"en-US\"><voice name=\"` +
			voiceName + `\"><mstts:express-as style=\"` + styleName + `\" styledegree=\"` + styleDegree + `\" role=\"` + roleName + `\"><prosody rate=\"{{(speakSpeed -10) * 2}}%\" pitch=\"+0Hz\">{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/\\/g, '')}}</prosody> </mstts:express-as></voice></speak>"}`
	}

	head := `{"Content-Type":"text/plain","Format":"` + voiceFormat + `", "Token":"` + token + `"}`
	legadoJson := &LegadoJson{Name: name, URL: url, ID: t, LastUpdateTime: t, ContentType: formatContentType(voiceFormat),
		Header: head, ConcurrentRate: concurrentRate}

	body, err := json.Marshal(legadoJson)
	if err != nil {
		return nil, err
	}

	return body, nil
}

const (
	textVar = "{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/\\/g, '')}}"
	rateVar = "{{(speakSpeed -10) * 2}}"
)

/* 生成阅读APP朗读引擎Json (Creation) */
func genLegadoCreationJson(api, name string, creationJson *CreationJson, token, concurrentRate string) ([]byte, error) {
	creationJson.Text = textVar
	creationJson.Rate = rateVar
	creationJson.Volume = "0"
	var jsonBuf bytes.Buffer
	encoder := json.NewEncoder(&jsonBuf)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(creationJson)
	if err != nil {
		return nil, err
	}

	t := time.Now().UnixNano() / 1e6 //毫秒时间戳
	//urlJsonStr := `{"text":"{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/\\/g, '')}}","voiceName":"` +
	//	pro.VoiceName + `","voiceId":"` + pro.VoiceId + `","rate":"{{(speakSpeed -10) * 2}}","style":"` + pro.Style + `","styleDegree":"` + styleDegree +
	//	`","role":"` + pro.Role + `","volume":"0%","format":"` + voiceFormat + `"}`
	url := api + `,{"method":"POST","body":` + string(jsonBuf.Bytes()) + `}`
	head := `{"Content-Type":"application/json", "Token":"` + token + `"}`

	legadoJson := &LegadoJson{Name: name, URL: url, ID: t, LastUpdateTime: t, ContentType: formatContentType(creationJson.Format),
		Header: head, ConcurrentRate: concurrentRate}
	body, err := json.Marshal(legadoJson)
	return body, err
}

/* 根据音频格式返回对应的Content-Type */
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
