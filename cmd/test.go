package cmd

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug().Msg("test called")
		test()
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

type Setting struct {
	SourcePath     string
	SourceFile     *os.File
	TempTargetPath string
	TempTargetFile *os.File
}

func test() {
	settings := []Setting{}
	sourcePath := ""

	paths := []string{
		"C:\\ProgramData\\Streambox\\SpectraUI\\settings.xml.bak",
		"C:\\ProgramData\\Streambox\\SpectraUI\\settings.xml",
		"data/settings.xml",
		"data/settings.xml.bak",
	}

	for _, pathStr := range paths {
		sourcePath, _ = filepath.Abs(pathStr)
		settings = append(settings, Setting{SourcePath: sourcePath})
	}

	for _, setting := range settings {
		updateSettings(setting)
	}
}

func updateSettings(s Setting) {
	s.TempTargetPath = filepath.Join(os.TempDir(), "nightrover_")
	if err := createTemporaryFile(&s); err != nil {
		log.Error().Msgf("Error creating temporary file %s: %v\n", s.TempTargetPath, err)
		return
	}
	defer os.Remove(s.TempTargetPath)

	log.Debug().Msgf("Temporary file %s created to update %s\n",
		s.TempTargetFile.Name(),
		s.SourcePath,
	)

	if err := openSourceFile(&s); err != nil {
		log.Error().Msgf("Failed to open file %s: %v\n", s.SourcePath, err)
		return
	}
	defer s.SourceFile.Close()

	pattern := regexp.MustCompile(`video_3d *= *"[^"]+" ?`)
	scanner := bufio.NewScanner(s.SourceFile)

	if err := processLines(scanner, pattern, s.TempTargetFile); err != nil {
		log.Error().Msgf("Error processing lines: %v\n", err)
		return
	}

	if sameChecksum, err := compareChecksums(s.TempTargetFile.Name(), s.SourceFile.Name()); err != nil {
		log.Debug().Msgf("Error comparing checksums: %v", err)
		return
	} else if sameChecksum {
		log.Debug().Msg("The files have the same checksum, no changes made.")
		return
	}

	if err := replaceOriginalFile(s.TempTargetFile.Name(), s.SourceFile.Name()); err != nil {
		log.Error().Msgf("Failed to replace the original file: %v\n", err)
		return
	}

	log.Info().Msgf("%s updated", s.SourcePath)
}

func createTemporaryFile(s *Setting) error {
	var err error
	s.TempTargetFile, err = os.CreateTemp(filepath.Dir(s.TempTargetPath), filepath.Base(s.TempTargetPath))
	return err
}

func openSourceFile(s *Setting) error {
	var err error
	s.SourceFile, err = os.Open(s.SourcePath)
	return err
}

func processLines(scanner *bufio.Scanner, pattern *regexp.Regexp, tempTargetFile *os.File) error {
	for scanner.Scan() {
		line := scanner.Text()
		newLine := pattern.ReplaceAllString(line, "")

		if _, err := fmt.Fprintln(tempTargetFile, newLine); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func compareChecksums(file1, file2 string) (bool, error) {
	sum1, err := calculateSHA256(file1)
	if err != nil {
		return false, err
	}

	sum2, err := calculateSHA256(file2)
	if err != nil {
		return false, err
	}

	return compareSums(sum1, sum2), nil
}

func calculateSHA256(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func compareSums(sum1, sum2 []byte) bool {
	return fmt.Sprintf("%x", sum1) == fmt.Sprintf("%x", sum2)
}

func replaceOriginalFile(tempTargetFile, sourceFile string) error {
	return os.Rename(tempTargetFile, sourceFile)
}
