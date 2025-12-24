package handlers

import (
	"archive/tar"
	"archive/zip"
	"bastion/database"
	"bastion/version"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/proxy"
)

const updateProxySettingKey = "update_proxy_url"

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

type releaseCache struct {
	mu       sync.Mutex
	release  *githubRelease
	etag     string
	fetched  time.Time
	lastErr  error
	lastCode int
}

var latestReleaseCache releaseCache

type updateCheckResponse struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url,omitempty"`
	AssetName       string `json:"asset_name,omitempty"`
	DownloadURL     string `json:"download_url,omitempty"`
}

type updateApplyResponse struct {
	OK            bool   `json:"ok"`
	TargetVersion string `json:"target_version"`
	Message       string `json:"message"`
	HelperPID     int    `json:"helper_pid,omitempty"`
	HelperLogPath string `json:"helper_log_path,omitempty"`
}

type updateGenerateCodeResponse struct {
	Code      string `json:"code"`
	ExpiresAt int64  `json:"expires_at"`
}

type updateProxyResponse struct {
	ManualProxy    string `json:"manual_proxy,omitempty"`
	EnvHTTPProxy   string `json:"env_http_proxy,omitempty"`
	EnvHTTPSProxy  string `json:"env_https_proxy,omitempty"`
	EnvAllProxy    string `json:"env_all_proxy,omitempty"`
	EnvNoProxy     string `json:"env_no_proxy,omitempty"`
	EffectiveProxy string `json:"effective_proxy,omitempty"`
	Source         string `json:"source"` // manual|env|none
}

type updateProxyRequest struct {
	ProxyURL string `json:"proxy_url"`
}

type updateApplyRequest struct {
	Code string `json:"code"`
}

type updateCodeManager struct {
	mu        sync.RWMutex
	code      string
	expiresAt time.Time
}

var updateMgr updateCodeManager

// CheckUpdate checks GitHub latest release and selects a matching asset for the current OS/arch.
func CheckUpdate(c *gin.Context) {
	log.Printf("update: check requested (client=%s)", c.ClientIP())
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		log.Printf("update: check fetch latest release failed: %v", err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Bad gateway", err.Error())
		return
	}

	assetName, downloadURL, err := selectReleaseAsset(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		log.Printf("update: check select asset failed (tag=%s os=%s arch=%s): %v", release.TagName, runtime.GOOS, runtime.GOARCH, err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Bad gateway", err.Error())
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

// GenerateUpdateCode creates a short-lived confirmation code for applying an update.
// A code is issued only when a newer GitHub "Latest Release" is available.
func GenerateUpdateCode(c *gin.Context) {
	log.Printf("update: generate code requested (client=%s)", c.ClientIP())
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		log.Printf("update: generate code fetch latest release failed: %v", err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Bad gateway", err.Error())
		return
	}

	current := strings.TrimSpace(version.Version)
	latest := strings.TrimSpace(release.TagName)
	if !isVersionNewer(latest, current) {
		log.Printf("update: generate code skipped (already up to date current=%s latest=%s)", current, latest)
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "already up to date")
		return
	}

	code, err := generateSixDigitCode()
	if err != nil {
		log.Printf("update: generate code failed: %v", err)
		errV2(c, http.StatusInternalServerError, CodeInternal, "Internal error", err.Error())
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

// GetUpdateProxy returns the current effective proxy configuration for update HTTP requests.
// "System" proxy is detected via environment variables (HTTP_PROXY/HTTPS_PROXY/NO_PROXY/ALL_PROXY).
func GetUpdateProxy(c *gin.Context) {
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

// SetUpdateProxy persists a manual proxy URL used by update HTTP requests. An empty value clears it.
func SetUpdateProxy(c *gin.Context) {
	var req updateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "Invalid request")
		return
	}

	value := strings.TrimSpace(req.ProxyURL)
	if value == "" {
		if err := database.DeleteSetting(updateProxySettingKey); err != nil {
			errV2(c, http.StatusInternalServerError, CodeInternal, "Internal error", err.Error())
			return
		}
		okV2(c, gin.H{"ok": true})
		return
	}

	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "invalid proxy url")
		return
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "proxy url must start with http(s):// or socks5(h)://")
		return
	}

	if err := database.SetSetting(updateProxySettingKey, value); err != nil {
		errV2(c, http.StatusInternalServerError, CodeInternal, "Internal error", err.Error())
		return
	}
	okV2(c, gin.H{"ok": true})
}

// ApplyUpdate downloads and installs the latest release asset, then restarts via a helper process.
func ApplyUpdate(c *gin.Context) {
	log.Printf("update: apply requested (client=%s)", c.ClientIP())
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	var req updateApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("update: apply invalid request: %v", err)
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "Invalid request")
		return
	}
	if shutdownChan == nil {
		log.Printf("update: apply aborted (shutdown channel not set)")
		errV2(c, http.StatusInternalServerError, CodeInternal, "Internal error", "shutdown channel is not initialized")
		return
	}
	if err := verifyUpdateCode(req.Code); err != nil {
		log.Printf("update: apply code verification failed: %v", err)
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", err.Error())
		return
	}

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		log.Printf("update: apply fetch latest release failed: %v", err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Bad gateway", err.Error())
		return
	}

	assetName, downloadURL, err := selectReleaseAsset(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		log.Printf("update: apply select asset failed (tag=%s os=%s arch=%s): %v", release.TagName, runtime.GOOS, runtime.GOARCH, err)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Bad gateway", err.Error())
		return
	}

	current := strings.TrimSpace(version.Version)
	latest := strings.TrimSpace(release.TagName)
	if !isVersionNewer(latest, current) {
		log.Printf("update: apply aborted (already up to date current=%s latest=%s)", current, latest)
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "already up to date")
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Printf("update: apply os.Executable failed: %v", err)
		errV2(c, http.StatusInternalServerError, CodeInternal, "Internal error", err.Error())
		return
	}
	exePath, _ = filepath.Abs(exePath)

	tmpDir, err := os.MkdirTemp("", "bastion-update-*")
	if err != nil {
		log.Printf("update: apply MkdirTemp failed: %v", err)
		errV2(c, http.StatusInternalServerError, CodeInternal, "Internal error", err.Error())
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
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Bad gateway", err.Error())
		return
	}

	newBinPath, err := extractBinary(archivePath, tmpDir, runtime.GOOS)
	if err != nil {
		log.Printf("update: apply extract failed (archive=%s tmp=%s): %v", archivePath, tmpDir, err)
		_ = os.RemoveAll(tmpDir)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Bad gateway", err.Error())
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
		// Even if the helper fails to open its own log file, its stderr/stdout will still be captured here.
		//
		// Note: do not close `f` here. The parent process exits shortly after starting the helper, and closing
		// the writer early would stop the stdout/stderr tee goroutines from writing.
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
		errV2(c, http.StatusInternalServerError, CodeInternal, "Internal error", err.Error())
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
		// Give the client time to receive and render the response before shutting down.
		time.Sleep(5 * time.Second)
		if shutdownChan != nil {
			log.Printf("update: apply triggering shutdown via channel")
			shutdownChan <- true
			return
		}
		log.Printf("update: apply cannot shutdown (shutdown channel not set)")
	}()
}

func ensureWritableFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_ = f.Close()
	return nil
}

func verifyUpdateCode(code string) error {
	code = strings.TrimSpace(code)

	updateMgr.mu.RLock()
	stored := updateMgr.code
	expiresAt := updateMgr.expiresAt
	updateMgr.mu.RUnlock()

	if stored == "" {
		return errors.New("no update code generated; please generate one first")
	}
	if time.Now().After(expiresAt) {
		updateMgr.mu.Lock()
		updateMgr.code = ""
		updateMgr.mu.Unlock()
		return errors.New("update code expired; please generate a new one")
	}
	if code != stored {
		return errors.New("invalid update code")
	}

	updateMgr.mu.Lock()
	updateMgr.code = ""
	updateMgr.mu.Unlock()
	return nil
}

func generateSixDigitCode() (string, error) {
	nBig, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", nBig.Int64()), nil
}

func fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	log.Printf("update: fetching latest release from GitHub API")

	// Small in-process cache to reduce GitHub API calls (helps avoid rate limits).
	const ttl = 30 * time.Second
	latestReleaseCache.mu.Lock()
	cached := latestReleaseCache.release
	etag := latestReleaseCache.etag
	fetched := latestReleaseCache.fetched
	latestReleaseCache.mu.Unlock()

	if cached != nil && time.Since(fetched) < ttl {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/wildking996/bastion/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "bastion-self-update")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		log.Printf("update: github auth enabled via env GITHUB_TOKEN")
	}

	client := newUpdateHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && cached != nil {
		latestReleaseCache.mu.Lock()
		latestReleaseCache.fetched = time.Now()
		latestReleaseCache.lastErr = nil
		latestReleaseCache.lastCode = resp.StatusCode
		latestReleaseCache.mu.Unlock()
		return cached, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		err := fmt.Errorf("github api error: %s: %s", resp.Status, strings.TrimSpace(string(body)))

		// If we have a cached release, try to degrade gracefully under rate limiting.
		if resp.StatusCode == http.StatusForbidden && cached != nil {
			log.Printf("update: github api forbidden, using cached release due to rate limiting: %v", err)
			return cached, nil
		}

		latestReleaseCache.mu.Lock()
		latestReleaseCache.lastErr = err
		latestReleaseCache.lastCode = resp.StatusCode
		latestReleaseCache.mu.Unlock()
		return nil, err
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	if release.TagName == "" {
		return nil, errors.New("github latest release missing tag_name")
	}

	if newEtag := strings.TrimSpace(resp.Header.Get("ETag")); newEtag != "" {
		etag = newEtag
	}

	latestReleaseCache.mu.Lock()
	latestReleaseCache.release = &release
	latestReleaseCache.etag = etag
	latestReleaseCache.fetched = time.Now()
	latestReleaseCache.lastErr = nil
	latestReleaseCache.lastCode = resp.StatusCode
	latestReleaseCache.mu.Unlock()

	return &release, nil
}

func selectReleaseAsset(release *githubRelease, goos, goarch string) (assetName, downloadURL string, err error) {
	if release == nil {
		return "", "", errors.New("nil release")
	}
	if len(release.Assets) == 0 {
		return "", "", errors.New("latest release has no assets")
	}

	osToken := strings.ToLower(goos)
	archToken := strings.ToLower(goarch)

	wantExt := ".tar.gz"
	if goos == "windows" {
		wantExt = ".zip"
	}

	var best struct {
		name string
		url  string
	}

	for _, a := range release.Assets {
		nameLower := strings.ToLower(a.Name)
		if !strings.Contains(nameLower, "bastion") {
			continue
		}
		if !strings.Contains(nameLower, osToken) {
			continue
		}
		if !strings.Contains(nameLower, archToken) {
			continue
		}
		if !strings.HasSuffix(nameLower, wantExt) {
			continue
		}

		tagLower := strings.ToLower(strings.TrimSpace(release.TagName))
		if tagLower != "" && strings.Contains(nameLower, tagLower) {
			best.name = a.Name
			best.url = a.BrowserDownloadURL
			break
		}

		if best.url == "" {
			best.name = a.Name
			best.url = a.BrowserDownloadURL
		}
	}

	if best.url == "" {
		return "", "", fmt.Errorf("no suitable asset found for %s/%s", goos, goarch)
	}
	return best.name, best.url, nil
}

func downloadFile(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	client := newUpdateHTTPClient(2 * time.Minute)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("download error: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(archivePath, tmpDir, goos string) (string, error) {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return extractZipBinary(archivePath, tmpDir, goos)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return extractTarGzBinary(archivePath, tmpDir, goos)
	}
	return "", fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
}

func extractZipBinary(path, tmpDir, goos string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	wantName := "bastion"
	if goos == "windows" {
		wantName = "bastion.exe"
	}

	var best *zip.File
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := strings.ToLower(filepath.Base(f.Name))
		if base == wantName {
			best = f
			break
		}
		if best == nil && strings.Contains(base, "bastion") {
			best = f
		}
	}
	if best == nil {
		return "", errors.New("zip archive does not contain bastion binary")
	}

	rc, err := best.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	outPath := filepath.Join(tmpDir, filepath.Base(wantName))
	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return "", err
	}

	return outPath, nil
}

func extractTarGzBinary(path, tmpDir, goos string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	wantName := "bastion"
	if goos == "windows" {
		wantName = "bastion.exe"
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		base := strings.ToLower(filepath.Base(hdr.Name))
		if base != wantName {
			continue
		}

		outPath := filepath.Join(tmpDir, filepath.Base(wantName))
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return "", err
		}
		_ = out.Close()
		return outPath, nil
	}

	return "", errors.New("tar.gz archive does not contain bastion binary")
}

func isVersionNewer(latest, current string) bool {
	lt := parseVersion(latest)
	ct := parseVersion(current)

	for i := 0; i < 3; i++ {
		if lt[i] > ct[i] {
			return true
		}
		if lt[i] < ct[i] {
			return false
		}
	}
	return false
}

func parseVersion(v string) [3]int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n := 0
		for _, ch := range parts[i] {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n
	}
	return out
}

func normalizeTag(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if v == "dev" || v == "unknown" {
		return v
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	if isLikelySemver(v) {
		return "v" + v
	}
	return v
}

func isLikelySemver(v string) bool {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		hasDigit := false
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return false
			}
			hasDigit = true
		}
		if !hasDigit {
			return false
		}
	}
	return true
}

func logProxyEnv(prefix string) {
	manual, _ := getManualUpdateProxyURL()
	httpProxy := strings.TrimSpace(os.Getenv("HTTP_PROXY"))
	if httpProxy == "" {
		httpProxy = strings.TrimSpace(os.Getenv("http_proxy"))
	}
	httpsProxy := strings.TrimSpace(os.Getenv("HTTPS_PROXY"))
	if httpsProxy == "" {
		httpsProxy = strings.TrimSpace(os.Getenv("https_proxy"))
	}
	noProxy := strings.TrimSpace(os.Getenv("NO_PROXY"))
	if noProxy == "" {
		noProxy = strings.TrimSpace(os.Getenv("no_proxy"))
	}
	allProxy := strings.TrimSpace(os.Getenv("ALL_PROXY"))
	if allProxy == "" {
		allProxy = strings.TrimSpace(os.Getenv("all_proxy"))
	}

	if manual == "" && httpProxy == "" && httpsProxy == "" && allProxy == "" && noProxy == "" {
		log.Printf("%s proxy: (none)", prefix)
		return
	}
	log.Printf(
		"%s proxy: manual=%s HTTP_PROXY=%s HTTPS_PROXY=%s ALL_PROXY=%s NO_PROXY=%s",
		prefix,
		redactProxy(manual),
		redactProxy(httpProxy),
		redactProxy(httpsProxy),
		redactProxy(allProxy),
		noProxy,
	)
}

func redactProxy(raw string) string {
	if raw == "" {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}
	parsed.User = nil
	return parsed.String()
}

type proxyEnv struct {
	httpProxy  string
	httpsProxy string
	allProxy   string
	noProxy    string
}

func readProxyEnv() proxyEnv {
	httpProxy := strings.TrimSpace(os.Getenv("HTTP_PROXY"))
	if httpProxy == "" {
		httpProxy = strings.TrimSpace(os.Getenv("http_proxy"))
	}
	httpsProxy := strings.TrimSpace(os.Getenv("HTTPS_PROXY"))
	if httpsProxy == "" {
		httpsProxy = strings.TrimSpace(os.Getenv("https_proxy"))
	}
	noProxy := strings.TrimSpace(os.Getenv("NO_PROXY"))
	if noProxy == "" {
		noProxy = strings.TrimSpace(os.Getenv("no_proxy"))
	}
	allProxy := strings.TrimSpace(os.Getenv("ALL_PROXY"))
	if allProxy == "" {
		allProxy = strings.TrimSpace(os.Getenv("all_proxy"))
	}
	return proxyEnv{
		httpProxy:  httpProxy,
		httpsProxy: httpsProxy,
		allProxy:   allProxy,
		noProxy:    noProxy,
	}
}

func chooseEffectiveProxy(manual string, env proxyEnv) (effective, source string) {
	if strings.TrimSpace(manual) != "" {
		return manual, "manual"
	}
	if strings.TrimSpace(env.httpsProxy) != "" {
		return env.httpsProxy, "env"
	}
	if strings.TrimSpace(env.httpProxy) != "" {
		return env.httpProxy, "env"
	}
	if strings.TrimSpace(env.allProxy) != "" {
		return env.allProxy, "env"
	}
	return "", "none"
}

func getManualUpdateProxyURL() (string, bool) {
	v, ok, err := database.GetSetting(updateProxySettingKey)
	if err != nil {
		return "", false
	}
	v = strings.TrimSpace(v)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

func newUpdateHTTPClient(timeout time.Duration) *http.Client {
	manual, _ := getManualUpdateProxyURL()
	env := readProxyEnv()
	effective, _ := chooseEffectiveProxy(manual, env)

	base, okType := http.DefaultTransport.(*http.Transport)
	var tr *http.Transport
	if okType {
		tr = base.Clone()
	} else {
		tr = &http.Transport{}
	}

	if strings.TrimSpace(effective) == "" {
		tr.Proxy = http.ProxyFromEnvironment
		return &http.Client{
			Timeout:   timeout,
			Transport: tr,
		}
	}

	pu, err := url.Parse(effective)
	if err != nil {
		tr.Proxy = http.ProxyFromEnvironment
		return &http.Client{
			Timeout:   timeout,
			Transport: tr,
		}
	}

	switch strings.ToLower(pu.Scheme) {
	case "http", "https":
		tr.Proxy = http.ProxyURL(pu)
	case "socks5", "socks5h":
		tr.Proxy = nil
		if strings.EqualFold(pu.Scheme, "socks5h") {
			pu.Scheme = "socks5"
		}
		dialer, err := proxy.FromURL(pu, proxy.Direct)
		if err == nil {
			tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				type dialContext interface {
					DialContext(context.Context, string, string) (net.Conn, error)
				}
				if dctx, ok := dialer.(dialContext); ok {
					return dctx.DialContext(ctx, network, addr)
				}
				return dialer.Dial(network, addr)
			}
		}
	default:
		tr.Proxy = http.ProxyFromEnvironment
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}
}
