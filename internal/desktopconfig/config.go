package desktopconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const configFileName = "damask.env"

// ConfigDir returns the OS-appropriate config directory for Damask, creating it if absent.
// If override is non-empty it is used as-is.
func ConfigDir(override string) (string, error) {
	if override != "" {
		if err := os.MkdirAll(override, 0o700); err != nil {
			return "", fmt.Errorf("desktopconfig: create config dir: %w", err)
		}
		return override, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("desktopconfig: UserConfigDir: %w", err)
	}
	dir := filepath.Join(base, "damask")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("desktopconfig: create config dir: %w", err)
	}
	return dir, nil
}

// ConfigFilePath returns the path to damask.env inside ConfigDir.
func ConfigFilePath(override string) (string, error) {
	dir, err := ConfigDir(override)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Exists reports whether damask.env is present.
func Exists(override string) (bool, error) {
	p, err := ConfigFilePath(override)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// Load reads damask.env into a map without setting env vars.
func Load(override string) (map[string]string, error) {
	p, err := ConfigFilePath(override)
	if err != nil {
		return nil, err
	}
	m, err := godotenv.Read(p)
	if err != nil {
		return nil, fmt.Errorf("desktopconfig: read %s: %w", p, err)
	}
	return m, nil
}

// Write atomically merges values into damask.env. Existing keys absent from values are preserved.
func Write(override string, values map[string]string) error {
	p, err := ConfigFilePath(override)
	if err != nil {
		return err
	}

	// Load existing so we can merge.
	existing := map[string]string{}
	if _, statErr := os.Stat(p); statErr == nil {
		existing, err = godotenv.Read(p)
		if err != nil {
			return fmt.Errorf("desktopconfig: read existing: %w", err)
		}
	}

	for k, v := range values {
		existing[k] = v
	}

	return atomicWrite(p, existing)
}

// atomicWrite serialises m to a .tmp file and renames it over dst.
func atomicWrite(dst string, m map[string]string) error {
	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("desktopconfig: open tmp: %w", err)
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		v := m[k]
		// Quote values that contain spaces or special chars.
		if strings.ContainsAny(v, " \t\n\"'\\") {
			v = `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(v)
		sb.WriteByte('\n')
	}

	if _, err := f.WriteString(sb.String()); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("desktopconfig: write tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("desktopconfig: close tmp: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("desktopconfig: rename: %w", err)
	}
	return nil
}

// BackupAndWipe copies damask.env to a timestamped backup, removes the original,
// and prunes old backups so at most maxBackups files are kept.
func BackupAndWipe(dir string, maxBackups int) error {
	src := filepath.Join(dir, configFileName)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil // nothing to backup
	}

	ts := time.Now().UTC().Format(time.RFC3339)
	ts = strings.ReplaceAll(ts, ":", "-")
	bak := filepath.Join(dir, configFileName+".bak."+ts)

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("desktopconfig: read for backup: %w", err)
	}
	if err := os.WriteFile(bak, data, 0o600); err != nil {
		return fmt.Errorf("desktopconfig: write backup: %w", err)
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("desktopconfig: remove original: %w", err)
	}

	return pruneBackups(dir, maxBackups)
}

// LatestBackup returns the parsed contents of the most recent backup file,
// or an empty map if none exist.
func LatestBackup(dir string) (map[string]string, error) {
	backups, err := listBackups(dir)
	if err != nil || len(backups) == 0 {
		return map[string]string{}, err
	}
	m, err := godotenv.Read(backups[len(backups)-1])
	if err != nil {
		return map[string]string{}, fmt.Errorf("desktopconfig: read backup: %w", err)
	}
	return m, nil
}

// listBackups returns backup file paths sorted oldest→newest.
func listBackups(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("desktopconfig: read dir: %w", err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), configFileName+".bak.") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

func pruneBackups(dir string, maxBackups int) error {
	backups, err := listBackups(dir)
	if err != nil {
		return err
	}
	for len(backups) > maxBackups {
		if err := os.Remove(backups[0]); err != nil {
			return fmt.Errorf("desktopconfig: prune backup: %w", err)
		}
		backups = backups[1:]
	}
	return nil
}
