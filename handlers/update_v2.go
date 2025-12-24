package handlers

import (
	"bastion/database"
	"bastion/version"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func CheckUpdateV2(c *gin.Context) {
	log.Printf("update: check requested (client=%s)", c.ClientIP())
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		log.Printf("update: check fetch latest release failed: %v", err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Failed to fetch latest release", err.Error())
		return
	}

	assetName, downloadURL, err := selectReleaseAsset(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		log.Printf("update: check select asset failed (tag=%s os=%s arch=%s): %v", release.TagName, runtime.GOOS, runtime.GOARCH, err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Failed to select release asset", err.Error())
		return
	}

	current := strings.TrimSpace(version.Version)
	latest := strings.TrimSpace(release.TagName)

	updateAvailable := isVersionNewer(latest, current)
	logProxyEnv("update: check")
	log.Printf(
		"update: check result current=%s latest=%s available=%v asset=%s",
		current, latest, updateAvailable, assetName,
	)

	okV2(c, updateCheckResponse{
		CurrentVersion:  normalizeTag(current),
		LatestVersion:   normalizeTag(latest),
		UpdateAvailable: updateAvailable,
		ReleaseURL:      release.HTMLURL,
		AssetName:       assetName,
		DownloadURL:     downloadURL,
	})
}

func GenerateUpdateCodeV2(c *gin.Context) {
	log.Printf("update: generate code requested (client=%s)", c.ClientIP())
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		log.Printf("update: generate code fetch latest release failed: %v", err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Failed to fetch latest release", err.Error())
		return
	}

	current := strings.TrimSpace(version.Version)
	latest := strings.TrimSpace(release.TagName)
	if !isVersionNewer(latest, current) {
		log.Printf("update: generate code skipped (already up to date current=%s latest=%s)", current, latest)
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Already up to date", "already up to date")
		return
	}

	code, err := generateSixDigitCode()
	if err != nil {
		log.Printf("update: generate code failed: %v", err)
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to generate code", err.Error())
		return
	}

	expiresAt := time.Now().Add(5 * time.Minute)
	updateMgr.mu.Lock()
	updateMgr.code = code
	updateMgr.expiresAt = expiresAt
	updateMgr.mu.Unlock()

	log.Printf("update: code generated (expires_at=%s)", expiresAt.UTC().Format(time.RFC3339))
	okV2(c, updateGenerateCodeResponse{
		Code:      code,
		ExpiresAt: expiresAt.Unix(),
	})
}

func GetUpdateProxyV2(c *gin.Context) {
	manual, _ := getManualUpdateProxyURL()
	env := readProxyEnv()
	effective, source := chooseEffectiveProxy(manual, env)

	okV2(c, updateProxyResponse{
		ManualProxy:    redactProxy(manual),
		EnvHTTPProxy:   redactProxy(env.httpProxy),
		EnvHTTPSProxy:  redactProxy(env.httpsProxy),
		EnvAllProxy:    redactProxy(env.allProxy),
		EnvNoProxy:     env.noProxy,
		EffectiveProxy: redactProxy(effective),
		Source:         source,
	})
}

func SetUpdateProxyV2(c *gin.Context) {
	var req updateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "invalid request")
		return
	}

	value := strings.TrimSpace(req.ProxyURL)
	if value == "" {
		if err := database.DeleteSetting(updateProxySettingKey); err != nil {
			errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to clear proxy", err.Error())
			return
		}
		okV2(c, gin.H{"ok": true})
		return
	}

	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid proxy url", "invalid proxy url")
		return
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid proxy url", "proxy url must start with http(s):// or socks5(h)://")
		return
	}

	if err := database.SetSetting(updateProxySettingKey, value); err != nil {
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to save proxy", err.Error())
		return
	}
	okV2(c, gin.H{"ok": true})
}

func ApplyUpdateV2(c *gin.Context) {
	log.Printf("update: apply requested (client=%s)", c.ClientIP())
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	var req updateApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("update: apply invalid request: %v", err)
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "Invalid request")
		return
	}
	if err := verifyUpdateCode(req.Code); err != nil {
		log.Printf("update: apply code verification failed: %v", err)
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid update code", err.Error())
		return
	}

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		log.Printf("update: apply fetch latest release failed: %v", err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Failed to fetch latest release", err.Error())
		return
	}

	assetName, downloadURL, err := selectReleaseAsset(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		log.Printf("update: apply select asset failed (tag=%s os=%s arch=%s): %v", release.TagName, runtime.GOOS, runtime.GOARCH, err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Failed to select release asset", err.Error())
		return
	}

	current := strings.TrimSpace(version.Version)
	latest := strings.TrimSpace(release.TagName)
	if !isVersionNewer(latest, current) {
		log.Printf("update: apply aborted (already up to date current=%s latest=%s)", current, latest)
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Already up to date", "already up to date")
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Printf("update: apply os.Executable failed: %v", err)
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to locate executable", err.Error())
		return
	}
	exePath, _ = filepath.Abs(exePath)

	tmpDir, err := os.MkdirTemp("", "bastion-update-*")
	if err != nil {
		log.Printf("update: apply MkdirTemp failed: %v", err)
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to create temp dir", err.Error())
		return
	}

	archivePath := filepath.Join(tmpDir, filepath.Base(assetName))
	logProxyEnv("update: apply")
	log.Printf(
		"update: apply downloading tag=%s asset=%s url=%s exe=%s tmp=%s archive=%s",
		latest, assetName, downloadURL, exePath, tmpDir, archivePath,
	)
	if err := downloadFile(ctx, downloadURL, archivePath); err != nil {
		log.Printf("update: apply download failed (dst=%s): %v", archivePath, err)
		_ = os.RemoveAll(tmpDir)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Failed to download update", err.Error())
		return
	}

	newBinPath, err := extractBinary(archivePath, tmpDir, runtime.GOOS)
	if err != nil {
		log.Printf("update: apply extract failed (archive=%s tmp=%s): %v", archivePath, tmpDir, err)
		_ = os.RemoveAll(tmpDir)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Failed to extract update", err.Error())
		return
	}
	log.Printf("update: apply extracted binary=%s", newBinPath)

	if runtime.GOOS != "windows" {
		_ = os.Chmod(newBinPath, 0o755)
	}

	helperLogPath := filepath.Join(filepath.Dir(exePath), "bastion-update-helper.log")
	if err := ensureWritableFile(helperLogPath); err != nil {
		helperLogPath = filepath.Join(os.TempDir(), fmt.Sprintf("bastion-update-helper-%d.log", time.Now().UnixNano()))
		if err2 := ensureWritableFile(helperLogPath); err2 != nil {
			log.Printf("update: apply create helper log file failed (path=%s): %v", helperLogPath, err2)
		}
	}

	helperArgs := []string{
		"--self-update-helper",
		"--target", exePath,
		"--source", newBinPath,
		"--parent-pid", fmt.Sprintf("%d", os.Getpid()),
		"--cleanup", tmpDir,
		"--helper-log", helperLogPath,
		"--restart",
		"--",
	}
	helperArgs = append(helperArgs, os.Args[1:]...)

	cmd := exec.Command(exePath, helperArgs...)
	if f, err := os.OpenFile(helperLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		cmd.Stdout = io.MultiWriter(os.Stdout, f)
		cmd.Stderr = io.MultiWriter(os.Stderr, f)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Printf("update: apply open helper log file for stdout/stderr failed (path=%s): %v", helperLogPath, err)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("update: apply start helper failed: %v", err)
		_ = os.RemoveAll(tmpDir)
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to start helper", err.Error())
		return
	}
	log.Printf("update: apply helper started (pid=%d) helper_log=%s", cmd.Process.Pid, helperLogPath)

	okV2(c, updateApplyResponse{
		OK:            true,
		TargetVersion: normalizeTag(latest),
		Message:       "update started; restarting",
		HelperPID:     cmd.Process.Pid,
		HelperLogPath: helperLogPath,
	})

	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}

	go func() {
		time.Sleep(5 * time.Second)
		if shutdownChan != nil {
			log.Printf("update: apply triggering shutdown via channel")
			shutdownChan <- true
			return
		}
		log.Printf("update: apply forcing os.Exit(0) (shutdown channel not set)")
		os.Exit(0)
	}()
}
