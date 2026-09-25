// config.go - configuration constants, collection maps, ROM systems, and setup/init helpers.
package app

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"godsend/models"
)

// ── Constants ─────────────────────────────────────────────────────────

const (
	Port            = "8080"
	MaxPartSize     = 1800000000
	MaxDLCSizeBytes = 349 * 1024 * 1024
	CopyBufferSize  = 4 * 1024 * 1024
	ServeBufferSize = 128 * 1024
	FTPPort         = 21
	FTPTimeout      = 30 * time.Second
	FTPBufferSize   = 1 * 1024 * 1024
	FTPMaxRetries   = 3
	FTPRetryDelay   = 2 * time.Second
	TCPSendBuffer   = 512 * 1024
	TCPKeepAlive    = 30 * time.Second

	IADownloadBase = "https://archive.org/download/"

	MinervaBrowseBase = "https://minerva-archive.org/browse/"

	IAChunkRetries       = 5
	IAChunkRetryBase     = 6 * time.Second
	IAParallelThreshold  = 32 * 1024 * 1024
	IASegmentSize        = 4 * 1024 * 1024
	IAParallelMaxDefault = 16
	IAParallelMaxCap     = 32
)

func ClampIAParallel(c int) int {
	if c < 1 { return 1 }
	if c > IAParallelMaxCap { return IAParallelMaxCap }
	return c
}

// ── Internet Archive Collection Map ───────────────────────────────────

var IACollections = map[string][]string{
	"xbox360": {
		"microsoft_xbox360_numberssymbols",
		"microsoft_xbox360_a_part1", "microsoft_xbox360_a_part2",
		"microsoft_xbox360_b_part1", "microsoft_xbox360_b_part2",
		"microsoft_xbox360_c_part1", "microsoft_xbox360_c_part2",
		"microsoft_xbox360_d_part1", "microsoft_xbox360_d_part2", "microsoft_xbox360_d_part3",
		"microsoft_xbox360_e",
		"microsoft_xbox360_f_part1", "microsoft_xbox360_f_part2",
		"microsoft_xbox360_g", "microsoft_xbox360_h", "microsoft_xbox360_i",
		"microsoft_xbox360_j", "microsoft_xbox360_k", "microsoft_xbox360_l",
		"microsoft_xbox360_m_part1", "microsoft_xbox360_m_part2",
		"microsoft_xbox360_n_part1", "microsoft_xbox360_n_part2",
		"microsoft_xbox360_o", "microsoft_xbox360_p", "microsoft_xbox360_q",
		"microsoft_xbox360_r", "microsoft_xbox360_s_part1", "microsoft_xbox360_s_part2",
		"microsoft_xbox360_t_part1", "microsoft_xbox360_t_part2",
		"microsoft_xbox360_u", "microsoft_xbox360_v", "microsoft_xbox360_w",
		"microsoft_xbox360_x_part1", "microsoft_xbox360_x_part2",
		"microsoft_xbox360_y", "microsoft_xbox360_z",
	},
	"xbox": {
		"microsoft_xbox_numberssymbols", "microsoft_xbox_a", "microsoft_xbox_b",
		"microsoft_xbox_c_part1", "microsoft_xbox_c_part2", "microsoft_xbox_d_part1", "microsoft_xbox_d_part2",
		"microsoft_xbox_e", "microsoft_xbox_f", "microsoft_xbox_g", "microsoft_xbox_h", "microsoft_xbox_i",
		"microsoft_xbox_j", "microsoft_xbox_k", "microsoft_xbox_l", "microsoft_xbox_m_part1", "microsoft_xbox_m_part2",
		"microsoft_xbox_n_part1", "microsoft_xbox_n_part2", "microsoft_xbox_o_part1", "microsoft_xbox_o_part2",
		"microsoft_xbox_p", "microsoft_xbox_q", "microsoft_xbox_r", "microsoft_xbox_s_part1", "microsoft_xbox_s_part2",
		"microsoft_xbox_t_part1", "microsoft_xbox_t_part2", "microsoft_xbox_u", "microsoft_xbox_v",
		"microsoft_xbox_w", "microsoft_xbox_x", "microsoft_xbox_y", "microsoft_xbox_z",
	},
	"digital": {"microsoft_xbox360_digital_part1", "microsoft_xbox360_digital_part2", "microsoft_xbox360_digital_part3", "microsoft_xbox360_digital_part4", "microsoft_xbox360_digital_part5", "microsoft_xbox360_digital_part6", "microsoft_xbox360_digital_part7"},
	"xbla": {"XBOX_360_XBLA"},
	"dlc": {"XBOX_360_DLC_1", "XBOX_360_DLC_2", "XBOX_360_DLC_3", "XBOX_360_DLC_4", "XBOX_360_DLC_5", "XBOX_360_DLC_6", "XBOX_360_XBLA_DLC"},
	"xblig": {"XBOX_360_XBLIG_1", "XBOX_360_XBLIG_2", "XBOX_360_XBLIG_3", "XBOX_360_XBLIG_4"},
	"games": {"XBOX_360_1", "XBOX_360_1_OTHER", "XBOX_360_2", "XBOX_360_3", "XBOX_360_4", "XBOX_360_5", "XBOX_360_6"},
}

var MinervaPageURLs = map[string]string{
	"xbox360": MinervaBrowseBase + "Redump/Microsoft%20-%20Xbox%20360/",
	"xbox":    MinervaBrowseBase + "Redump/Microsoft%20-%20Xbox/",
	"digital": MinervaBrowseBase + "No-Intro/Microsoft%20-%20Xbox%20360%20(Digital)/",
	"xbla":    MinervaBrowseBase + "No-Intro/Microsoft%20-%20Xbox%20360%20(Digital)/",
	"dlc":     MinervaBrowseBase + "No-Intro/Microsoft%20-%20Xbox%20360%20(Digital)/",
	"xblig":   MinervaBrowseBase + "No-Intro/Microsoft%20-%20Xbox%20360%20(Digital)/",
	"games":   MinervaBrowseBase + "No-Intro/Non-Redump%20-%20Microsoft%20-%20Xbox%20360/",
}

var MinervaTagFilters = map[string][]string{
	"xbla":  {"(XBLA)"},
	"dlc":   {"(Addon)", "(DLC)", "(Addon for XBLA)"},
	"xblig": {"(XBLIG)"},
}

const MinervaCacheSchema = 2

var MinervaTorrentURLs = map[string]string{
	"xbox360": "https://minerva-archive.org/assets/Minerva_Myrient_v0.3/Minerva_Myrient%20-%20Redump%20-%20Microsoft%20-%20Xbox%20360.torrent",
	"xbox":    "https://minerva-archive.org/assets/Minerva_Myrient_v0.3/Minerva_Myrient%20-%20Redump%20-%20Microsoft%20-%20Xbox.torrent",
	"digital": "https://minerva-archive.org/assets/Minerva_Myrient_v0.3/Minerva_Myrient%20-%20No-Intro%20-%20Microsoft%20-%20Xbox%20360%20(Digital).torrent",
	"xbla":    "https://minerva-archive.org/assets/Minerva_Myrient_v0.3/Minerva_Myrient%20-%20No-Intro%20-%20Microsoft%20-%20Xbox%20360%20(Digital).torrent",
	"dlc":     "https://minerva-archive.org/assets/Minerva_Myrient_v0.3/Minerva_Myrient%20-%20No-Intro%20-%20Microsoft%20-%20Xbox%20360%20(Digital).torrent",
	"xblig":   "https://minerva-archive.org/assets/Minerva_Myrient_v0.3/Minerva_Myrient%20-%20No-Intro%20-%20Microsoft%20-%20Xbox%20360%20(Digital).torrent",
	"games":   "https://minerva-archive.org/assets/Minerva_Myrient_v0.3/Minerva_Myrient%20-%20No-Intro%20-%20Non-Redump%20-%20Microsoft%20-%20Xbox%20360.torrent",
}

var MinervaHrefRe = regexp.MustCompile(`href="(/rom\?name=[^"]+)"`)
var MinervaRomIDLinkRe = regexp.MustCompile(`(?i)href="/rom\?id=\d+"[^>]*>([^<]+\.(?:zip|7z|rar))</a>`)
var MinervaDataNameRe = regexp.MustCompile(`(?i)data-name="([^"]+\.(?:zip|7z|rar))"`)

// Setup helpers

func (a *App) SetupPaths() error {
	ex, err := os.Executable()
	if err != nil { return fmt.Errorf("executable path: %w", err) }
	exDir := filepath.Dir(ex)
	a.GodsendExeDir = exDir
	if v := strings.TrimSpace(os.Getenv("GODSEND_HOME")); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil { return fmt.Errorf("GODSEND_HOME: %w", err) }
		a.ToolsDir = abs
		a.Logf("[INFO] Data directory (GODSEND_HOME): %s", a.ToolsDir)
		a.Logf("[INFO] Executable: %s", ex)
	} else {
		a.ToolsDir = exDir
	}
	for _, dir := range []string{"Ready", "Temp", "cache"} {
		if err := os.MkdirAll(filepath.Join(a.ToolsDir, dir), 0755); err != nil { return err }
	}
	a.TorrentTempDir = filepath.Join(a.ToolsDir, "Temp", "torrent-dl")
	if v := strings.TrimSpace(os.Getenv("GODSEND_TORRENT_TEMP")); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil { return fmt.Errorf("GODSEND_TORRENT_TEMP: %w", err) }
		a.TorrentTempDir = abs
		a.Logf("[INFO] Torrent download temp (GODSEND_TORRENT_TEMP): %s", a.TorrentTempDir)
	}
	if err := os.MkdirAll(a.TorrentTempDir, 0755); err != nil { return fmt.Errorf("torrent temp dir: %w", err) }

	a.GODOutputDir = filepath.Join(a.ToolsDir, "GOD")
	if v := strings.TrimSpace(os.Getenv("GODSEND_GOD_OUTPUT")) ; v != "" {
		abs, err := filepath.Abs(v)
		if err != nil { return fmt.Errorf("GODSEND_GOD_OUTPUT: %w", err) }
		a.GODOutputDir = abs
		a.Logf("[INFO] Local GOD output: %s", a.GODOutputDir)
	}
	if err := os.MkdirAll(a.GODOutputDir, 0755); err != nil { return fmt.Errorf("GOD output dir: %w", err) }

	a.ROMRootPath = "Emulators\RetroArch\roms"
	if v := strings.TrimSpace(os.Getenv("GODSEND_ROM_PATH")); v != "" {
		v = strings.ReplaceAll(v, "/", "\")
		a.ROMRootPath = strings.TrimRight(v, "\")
	}

	a.TransferDir = filepath.Join(a.ToolsDir, "Transfer")
	if v := strings.TrimSpace(os.Getenv("GODSEND_TRANSFER")); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil { return fmt.Errorf("GODSEND_TRANSFER: %w", err) }
		a.TransferDir = abs
		a.Logf("[INFO] Local Transfer folder (GODSEND_TRANSFER): %s", a.TransferDir)
	}
	if err := os.MkdirAll(a.TransferDir, 0755); err != nil { return err }

	a.SaveBackupDir = a.TransferDir
	if v := strings.TrimSpace(os.Getenv("GODSEND_SAVE_BACKUP")); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil { return fmt.Errorf("GODSEND_SAVE_BACKUP: %w", err) }
		a.SaveBackupDir = abs
		a.Logf("[INFO] Save backup folder (GODSEND_SAVE_BACKUP): %s", a.SaveBackupDir)
	}
	a.ServerPort = Port
	if v := strings.TrimSpace(os.Getenv("GODSEND_PORT")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 65535 { return fmt.Errorf("GODSEND_PORT must be an integer between 1 and 65535") }
		a.ServerPort = strconv.Itoa(n)
		a.Logf("[INFO] Server port (GODSEND_PORT): %s", a.ServerPort)
	}
	a.FTPUsername = "xboxftp"
	a.FTPPassword = "xboxftp"
	if v := strings.TrimSpace(os.Getenv("GODSEND_FTP_USER")); v != "" { a.FTPUsername = v }
	if v := os.Getenv("GODSEND_FTP_PASS"); v != "" { a.FTPPassword = v }
	a.PendingFTPDir = filepath.Join(a.ToolsDir, "pending_ftp")
	os.MkdirAll(a.PendingFTPDir, 0755)

	a.DefaultXboxDrive = strings.TrimSpace(os.Getenv("GODSEND_DEFAULT_DRIVE"))
	a.CustomGodPath = strings.TrimSpace(os.Getenv("GODSEND_CUSTOM_GOD_PATH"))
	a.CustomXexPath = strings.TrimSpace(os.Getenv("GODSEND_CUSTOM_XEX_PATH"))
	a.Aria2ListenPort = strings.TrimSpace(os.Getenv("GODSEND_ARIA2_LISTEN_PORT"))
	a.Aria2DhtPort = strings.TrimSpace(os.Getenv("GODSEND_ARIA2_DHT_PORT"))
	a.CleanupEmptyReadyDirs()
	return nil
}

func (a *App) LoadIAAuthFromEnv() {
	v := strings.TrimSpace(os.Getenv("GODSEND_IA_COOKIE"))
	if len(v) > 7 && strings.EqualFold(v[:7], "cookie:") { v = strings.TrimSpace(v[7:]) }
	v = strings.ReplaceAll(strings.ReplaceAll(v, "\r", ""), "\n", "")
	a.IACookieHeader = strings.TrimSpace(v)
	aa := strings.TrimSpace(os.Getenv("GODSEND_IA_AUTHORIZATION"))
	if len(aa) > 14 && strings.EqualFold(aa[:14], "authorization:") { aa = strings.TrimSpace(aa[14:]) }
	a.IAAuthorizationHeader = strings.TrimSpace(aa)
	a.IADownloadMaxParallel = IAParallelMaxDefault
	if v := strings.TrimSpace(os.Getenv("GODSEND_IA_MAX_CONNECTIONS")); v != "" {
		if c, err := strconv.Atoi(v); err == nil { a.IADownloadMaxParallel = ClampIAParallel(c) }
	} else if v := strings.TrimSpace(os.Getenv("GODSEND_IA_CONCURRENCY")); v != "" {
		if c, err := strconv.Atoi(v); err == nil { a.IADownloadMaxParallel = ClampIAParallel(c) }
	}
	a.IAHTTPClient = &http.Client{Timeout: 0, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 { return fmt.Errorf("too many redirects") }
		if len(via) > 0 {
			for key, vals := range via[0].Header {
				if _, set := req.Header[key]; !set { req.Header[key] = vals }
			}
		}
		return nil
	}}
	if a.IACookieHeader != "" { a.Logf("[INFO] Internet Archive: Cookie header set (%d chars)", len(a.IACookieHeader)) }
	if a.IAAuthorizationHeader != "" { a.Logf("[INFO] Internet Archive: Authorization header set (%d chars)", len(a.IAAuthorizationHeader)) }
	a.Logf("[INFO] Internet Archive: chunked HTTP downloads (max %d parallel range requests)", a.IADownloadMaxParallel)
}
