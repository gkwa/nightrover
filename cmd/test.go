package cmd

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"

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
		log.Println("test called")
		test()
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}

type Settings struct {
	SourcePath     string
	SourceFile     *os.File
	TempTargetPath string
	TempTargetFile *os.File
}

func test() {
	settings := []Settings{}
	sourcePath := ""

	paths := []string{
		"C:\\ProgramData\\Streambox\\SpectraUI\\settings.xml.bak",
		"C:\\ProgramData\\Streambox\\SpectraUI\\settings.xml",
		"data/settings.xml",
		"data/settings.xml.bak",
	}

	for _, pathStr := range paths {
		sourcePath, _ = filepath.Abs(pathStr)
		settings = append(settings, Settings{SourcePath: sourcePath})
	}

	for _, thing := range settings {
		updateSettings(thing)
	}
}

func updateSettings(x1 Settings) {
	x1.TempTargetPath = filepath.Join(os.TempDir(), "nightrover_")
	if err := createTemporaryFile(&x1); err != nil {
		log.Printf("Error creating temporary file %s: %v\n", x1.TempTargetPath, err)
		return
	}
	defer os.Remove(x1.TempTargetPath)

	log.Printf("Temporary file %s created to update %s\n",
		x1.TempTargetFile.Name(),
		x1.SourcePath,
	)

	if err := openSourceFile(&x1); err != nil {
		log.Printf("Failed to open file %s: %v\n", x1.SourcePath, err)
		return
	}
	defer x1.SourceFile.Close()

	pattern := regexp.MustCompile(`video_3d *= *"[^"]+" ?`)
	scanner := bufio.NewScanner(x1.SourceFile)

	if err := processLines(scanner, pattern, x1.TempTargetFile); err != nil {
		log.Printf("Error processing lines: %v\n", err)
		return
	}

	if sameChecksum, err := compareChecksums(x1.TempTargetFile.Name(), x1.SourceFile.Name()); err != nil {
		log.Println("Error comparing checksums:", err)
		return
	} else if sameChecksum {
		log.Println("The files have the same checksum, no changes made.")
		return
	}

	if err := replaceOriginalFile(x1.TempTargetFile.Name(), x1.SourceFile.Name()); err != nil {
		log.Printf("Failed to replace the original file: %v\n", err)
		return
	}

	log.Println("Replacement successful!")
}

func createTemporaryFile(x1 *Settings) error {
	var err error
	x1.TempTargetFile, err = os.CreateTemp(filepath.Dir(x1.TempTargetPath), filepath.Base(x1.TempTargetPath))
	return err
}

func openSourceFile(x1 *Settings) error {
	var err error
	x1.SourceFile, err = os.Open(x1.SourcePath)
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
