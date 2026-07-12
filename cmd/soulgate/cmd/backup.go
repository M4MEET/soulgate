package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup SoulGate configuration and data",
	Long: `Create a compressed backup of SoulGate configuration, sessions, and memory.

Backs up:
  - .soulgate/ (config, policy, memory, audit logs, vectors)
  - sessions/ (gateway session recordings)

Examples:
  soulgate backup                          # Backup to soulgate-backup-YYYY-MM-DD.tar.gz
  soulgate backup --output my-backup.tar.gz
  soulgate backup --include-sessions=false  # Skip session recordings`,
	Args: cobra.NoArgs,
	RunE: runBackup,
}

var (
	backupOutput          string
	backupIncludeSessions bool
)

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.Flags().StringVarP(&backupOutput, "output", "o", "", "Output file path (default: soulgate-backup-YYYY-MM-DD.tar.gz)")
	backupCmd.Flags().BoolVar(&backupIncludeSessions, "include-sessions", true, "Include session recordings in backup")
}

func runBackup(cmd *cobra.Command, args []string) error {
	if backupOutput == "" {
		backupOutput = fmt.Sprintf("soulgate-backup-%s.tar.gz", time.Now().Format("2006-01-02"))
	}

	homeDir, _ := os.UserHomeDir()
	dirs := []string{
		".soulgate",
		filepath.Join(homeDir, ".soulgate"),
	}
	if backupIncludeSessions {
		dirs = append(dirs, "sessions")
	}

	outFile, err := os.Create(backupOutput)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer outFile.Close()

	gz := gzip.NewWriter(outFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	fileCount := 0
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip files we can't read
			}
			if info.IsDir() {
				return nil
			}
			// Skip large binary files
			if info.Size() > 100*1024*1024 {
				return nil
			}
			// Skip .git
			if strings.Contains(path, ".git") {
				return nil
			}

			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return nil
			}
			header.Name = path

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			io.Copy(tw, f)
			fileCount++
			return nil
		})
		if err != nil {
			return fmt.Errorf("backup walk error: %w", err)
		}
	}

	fmt.Printf("  ✓ Backup created: %s (%d files)\n", backupOutput, fileCount)
	return nil
}
