package cmd

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nightrover",
	Short: "Update spectra settings.xml",
	Long:  `Update spectra settings.xml for https://basecamp.com/2498935/projects/17265419/todos/449348526#comment_896648755`,
	Run: func(cmd *cobra.Command, args []string) {
		doit()
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("log-level",
		"info", "Log level (debug, info, warn, error, fatal, panic)",
	)

	consoleWriter := zerolog.ConsoleWriter{
		Out: os.Stderr,
		PartsExclude: []string{
			zerolog.TimestampFieldName,
		},
		FormatLevel: func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("[%s]", i))
		},
		FormatCaller: func(i interface{}) string {
			return filepath.Base(fmt.Sprintf("%s", i))
		},
	}

	log.Logger = log.Output(consoleWriter).With().Caller().Logger()
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".nightrover" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".nightrover")
	}

	viper.AutomaticEnv() // read in environment variables that match

	viper.BindPFlag("log-level", rootCmd.Flags().Lookup("log-level"))

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	setupLogging()
}

func setupLogging() {
	logLevel := viper.GetString("log-level")

	switch logLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

type Setting struct {
	SourcePath     string
	SourceFile     *os.File
	TempTargetPath string
	TempTargetFile *os.File
}

func doit() {
	paths := []string{}

	switch os := runtime.GOOS; os {
	case "darwin", "linux":
		// for testing
		paths = append(paths, "data/settings.xml.bak")
		paths = append(paths, "data/settings.xml")
	case "windows":
		paths = append(paths, "C:\\ProgramData\\Streambox\\SpectraUI\\settings.xml.bak")
		paths = append(paths, "C:\\ProgramData\\Streambox\\SpectraUI\\settings.xml")
	default:
		log.Error().Msgf("Running on %s.\n", os)
	}

	settings := []Setting{}
	sourcePath := ""
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
		log.Info().Msgf("%s is already updated", s.SourceFile.Name())
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
