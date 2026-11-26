package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/fileutil"
	"github.com/emuyx/cubedock/internal/m4399login"
)

// saveCaptchaImage writes a 4399 captcha challenge into the data directory and
// returns the path plus a user-facing note. The terminal cannot render the
// image, so the operator opens it with the system viewer (Ctrl+O).
func saveCaptchaImage(manager *core.Manager, captcha *m4399login.CaptchaRequired) (string, string) {
	note := strings.TrimSpace(captcha.Message)
	if note == "" {
		note = "请输入图形验证码"
	}
	if len(captcha.Image) == 0 {
		return "", note + "；没有收到验证码图片，请改用 WebUI 登录"
	}
	ext := ".png"
	imageType := strings.ToLower(captcha.ImageType)
	if strings.Contains(imageType, "jpeg") || strings.Contains(imageType, "jpg") {
		ext = ".jpg"
	}
	name := fmt.Sprintf("cubedock-4399-captcha-%s-%d%s",
		time.Now().Format("20060102-150405"), time.Now().UnixNano()%1e6, ext)
	path := filepath.Join(manager.Dir(), name)
	if err := fileutil.Replace(path, captcha.Image, 0600); err != nil {
		return "", note + "；保存验证码图片失败：" + err.Error()
	}
	return path, note + "；图片已保存，Ctrl+O 打开"
}
