// local_god.go - local-only Minerva GOD output helpers.
package pipeline

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"godsend/infrastructure/helpers"
	"godsend/models"
	"godsend/utils"
)

type localTorrentJob struct {
	GameName string `json:"game_name"`
	Platform string `json:"platform"`
	Entry models.MinervaEntry `json:"entry"`
}

func (s *Service) localTorrentJobPath(gameName string) string {
	safe := helpers.SanitizeFilename(gameName)
	return filepath.Join(s.App.TorrentTempDir, "local-jobs", safe+".json")
}

func (s *Service) saveLocalTorrentJob(gameName, platform string, entry models.MinervaEntry) error {
	dir := filepath.Join(s.App.TorrentTempDir, "local-jobs")
	if err := os.MkdirAll(dir, 0755); err != nil { return err }
	b, err := json.MarshalIndent(localTorrentJob{GameName:gameName, Platform:platform, Entry:entry}, "", "  ")
	if err != nil { return err }
	return os.WriteFile(s.localTorrentJobPath(gameName), b, 0644)
}

func (s *Service) removeLocalTorrentJob(gameName string) {
	_ = os.Remove(s.localTorrentJobPath(gameName))
}

// ResumePersistedLocalGames resumes local Minerva jobs that were active when GODsend stopped.
func (s *Service) ResumePersistedLocalGames() {
	dir := filepath.Join(s.App.TorrentTempDir, "local-jobs")
	entries, err := os.ReadDir(dir)
	if err != nil { return }
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" { continue }
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil { continue }
		var job localTorrentJob
		if json.Unmarshal(b, &job) != nil || job.GameName == "" || job.Entry.FileName == "" { continue }
		s.App.Logf("TORRENT PENDING: Found local GOD download for %s", job.GameName)
		if s.Torrent.IsPaused(job.GameName) {
			s.App.LogStatus(job.GameName, "Paused", "Paused from previous session")
			continue
		}
		s.App.LogStatus(job.GameName, "Queued", "Resuming previous torrent...")
		go s.ProcessMinervaLocalGame(job.GameName, job.Entry, job.Platform)
	}
}

// ResumePersistedLocalGame starts one persisted local download, used by the Queue Resume button after a backend restart.
func (s *Service) ResumePersistedLocalGame(gameName string) error {
	path := s.localTorrentJobPath(gameName)
	b, err := os.ReadFile(path)
	if err != nil { return fmt.Errorf("no persisted local torrent for %q", gameName) }
	var job localTorrentJob
	if err := json.Unmarshal(b, &job); err != nil { return fmt.Errorf("read persisted torrent: %w", err) }
	_ = os.Remove(filepath.Join(s.App.TorrentTempDir, "local-jobs", helpers.SanitizeFilename(gameName)+".paused"))
	go s.ProcessMinervaLocalGame(job.GameName, job.Entry, job.Platform)
	return nil
}

// ProcessMinervaLocalGame converts a Minerva disc release to a local GOD tree.
// It never consults the Xbox connection and never schedules FTP.
func (s *Service) ProcessMinervaLocalGame(gameName string, entry models.MinervaEntry, platform string) {
	safeName := helpers.SanitizeFilename(gameName)
	if safeName == "" {
		s.App.LogStatus(gameName, "Error", "Invalid game name")
		return
	}

	outputRoot := strings.TrimSpace(s.App.GODOutputDir)
	if outputRoot == "" {
		s.App.LogStatus(gameName, "Error", "No GOD output folder configured")
		return
	}
	gameDir := filepath.Join(outputRoot, safeName)
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		s.App.LogStatus(gameName, "Error", fmt.Sprintf("Create output: %v", err))
		return
	}

	torrentDir := filepath.Join(s.App.TorrentTempDir, safeName)
	if err := os.MkdirAll(torrentDir, 0755); err != nil {
		s.App.LogStatus(gameName, "Error", fmt.Sprintf("Create torrent staging: %v", err))
		return
	}
	defer os.RemoveAll(torrentDir)

	// A previous normal pipeline may have left a persisted FTP retry for this
	// same game. Local Minerva owns the job now, so remove those stale entries.
	for _, ftpJob := range s.FTP.LoadAllPendingFTPJobs() {
		if ftpJob.GameName == gameName {
			s.FTP.DeletePendingFTPJob(ftpJob.ID)
			s.App.Logf("LOCAL GOD: removed stale FTP retry for %s", gameName)
		}
	}

	if err := s.saveLocalTorrentJob(gameName, platform, entry); err != nil {
		s.App.LogStatus(gameName, "Error", fmt.Sprintf("Persist torrent job: %v", err))
		return
	}
	s.App.Logf("=== Minerva Local GOD: %s (%s) ===", gameName, platform)
	s.App.Logf("LOCAL GOD [%s]: [DEBUG] outputRoot=%s gameDir=%s torrentDir=%s entry=%q", gameName, outputRoot, gameDir, torrentDir, entry.FileName)
	s.App.LogStatus(gameName, "Processing", "Starting Minerva torrent download...")
	s.App.Logf("LOCAL GOD [%s]: [DEBUG] calling DownloadViaTorrent...", gameName)
	archivePath, err := s.Torrent.DownloadViaTorrent(platform, torrentDir, gameName, entry, s.debridTorrentDownloader(gameName))
	if err != nil {
		s.App.Logf("LOCAL GOD [%s]: [DEBUG] DownloadViaTorrent failed: %v", gameName, err)
		s.App.LogStatus(gameName, "Error", fmt.Sprintf("Minerva torrent: %v", err))
		return
	}
	s.App.Logf("LOCAL GOD [%s]: [DEBUG] torrent returned archive=%s", gameName, archivePath)
	if st, statErr := os.Stat(archivePath); statErr == nil {
		s.App.Logf("LOCAL GOD [%s]: [DEBUG] archive size=%d bytes", gameName, st.Size())
	} else {
		s.App.Logf("LOCAL GOD [%s]: [DEBUG] archive stat failed: %v", gameName, statErr)
	}

	extDir := filepath.Join(s.App.ToolsDir, "Temp", safeName+"_local_ext")
	os.RemoveAll(extDir)
	defer os.RemoveAll(extDir)
	s.App.LogStatus(gameName, "Processing", "Extracting archive...")
	s.App.Logf("LOCAL GOD [%s]: [DEBUG] extracting %s -> %s", gameName, archivePath, extDir)

	if err := utils.ExtractArchive(archivePath, extDir); err != nil {
		s.App.LogStatus(gameName, "Error", fmt.Sprintf("Extract: %v", err))
		return
	}
	s.App.Logf("LOCAL GOD [%s]: [DEBUG] archive extraction completed", gameName)

	// Minerva Redump is normally an archive containing one ISO. Convert it
	// directly into <output>/<game>/<TitleID>/00007000/... .
	isoPath := findFileByExt(extDir, ".iso")
	s.App.Logf("LOCAL GOD [%s]: [DEBUG] ISO search result=%q", gameName, isoPath)
	if isoPath != "" {
		s.App.LogStatus(gameName, "Processing", "Converting ISO to GOD...")
		s.App.Logf("LOCAL GOD [%s]: [DEBUG] RunIso2GodNative input=%s output=%s", gameName, isoPath, gameDir)
		if err := utils.RunIso2GodNative(isoPath, gameDir, Iso2GodResolveDisplayTitle); err != nil {
			s.App.LogStatus(gameName, "Error", fmt.Sprintf("GOD convert: %v", err))
			return
		}
		titleID, mediaID, err := helpers.DetectGodStructure(gameDir)
		if err != nil {
			s.App.LogStatus(gameName, "Error", fmt.Sprintf("GOD detect: %v", err))
			return
		}
		s.App.Logf("LOCAL GOD [%s]: [DEBUG] DetectGodStructure TitleID=%s MediaID=%s", gameName, titleID, mediaID)
		s.App.Logf("Local GOD complete: TitleID=%s MediaID=%s", titleID, mediaID)
		s.removeLocalTorrentJob(gameName)
		s.App.LogStatus(gameName, "Ready", "GOD ready")
		s.App.Logf("=== Complete (Minerva Local GOD): %s ===", gameName)
		return
	}

	// Some Minerva releases are already GOD/MGOD. Preserve the existing GOD
	// structure beneath the game-name folder rather than reconverting it.
	titleID, mediaID, srcRoot, err := findGODRoot(extDir)
	s.App.Logf("LOCAL GOD [%s]: [DEBUG] existing GOD search titleID=%s mediaID=%s root=%q err=%v", gameName, titleID, mediaID, srcRoot, err)
	if err == nil {
		os.RemoveAll(gameDir)
		if err := copyTree(srcRoot, gameDir); err != nil {
			s.App.LogStatus(gameName, "Error", fmt.Sprintf("Copy GOD: %v", err))
			return
		}
		s.App.Logf("Local GOD reused: TitleID=%s MediaID=%s", titleID, mediaID)
		s.removeLocalTorrentJob(gameName)
		s.App.LogStatus(gameName, "Ready", "GOD ready")
		s.App.Logf("=== Complete (Minerva Local GOD existing): %s ===", gameName)
		return
	}

	s.App.LogStatus(gameName, "Error", "No ISO or valid GOD structure found in Minerva archive")
}


func findFileByExt(root, ext string) string {
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() { return nil }
		if strings.EqualFold(filepath.Ext(path), ext) { found = path; return filepath.SkipAll }
		return nil
	})
	return found
}

func isHexString(s string) bool {
	if len(s) == 0 { return false }
	for _, r := range s {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') && !(r >= 'A' && r <= 'F') { return false }
	}
	return true
}

// findGODRoot locates the directory whose immediate children contain
// <TitleID>/<ContentType> with a non-DATA metadata file.
func findGODRoot(root string) (string, string, string, error) {
	var foundRoot, titleID, mediaID string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || !info.IsDir() {
			return nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}
		for _, titleEntry := range entries {
			if !titleEntry.IsDir() || len(titleEntry.Name()) != 8 || !isHexString(titleEntry.Name()) {
				continue
			}
			titlePath := filepath.Join(path, titleEntry.Name())
			cts, err := os.ReadDir(titlePath)
			if err != nil {
				continue
			}
			for _, ct := range cts {
				if !ct.IsDir() || len(ct.Name()) != 8 || !isHexString(ct.Name()) {
					continue
				}
				ctPath := filepath.Join(titlePath, ct.Name())
				files, err := os.ReadDir(ctPath)
				if err != nil {
					continue
				}
				for _, f := range files {
					if f.IsDir() || strings.HasPrefix(strings.ToUpper(f.Name()), "DATA") {
						continue
					}
					foundRoot = path
					titleID = strings.ToUpper(titleEntry.Name())
					mediaID = strings.ToUpper(f.Name())
					return io.EOF
				}
			}
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return "", "", "", err
	}
	if foundRoot == "" {
		return "", "", "", fmt.Errorf("GOD structure not found")
	}
	return titleID, mediaID, foundRoot, nil
}

// copyTree copies a complete directory tree.
func copyTree(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		out := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(out, 0755)
		}
		return helpers.CopyFileBuffered(path, out)
	})
}

// ResolveLocalGameFolder returns the folder used for a completed local game.
func (s *Service) ResolveLocalGameFolder(gameName string) string {
	safe := helpers.SanitizeFilename(gameName)
	if safe == "" || s.App.GODOutputDir == "" {
		return ""
	}
	return filepath.Join(s.App.GODOutputDir, safe)
}

